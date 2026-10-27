package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"meno/internal/api"
	"meno/internal/client"
	"meno/internal/config"
)

var errStopStream = errors.New("stop")

func newClientID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// headlessResult is what --json prints.
type headlessResult struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// runHeadless runs one prompt in a session of its own and prints the answer.
func runHeadless(l launch, stdin io.Reader, out, errOut io.Writer) int {
	prompt, err := readPrompt(l, stdin)
	if err != nil {
		fmt.Fprintln(errOut, "meno:", err)
		return 2
	}
	paths, err := config.DefaultPaths()
	if err != nil {
		fmt.Fprintln(errOut, "meno:", err)
		return 1
	}
	h, err := startHost(paths.Home)
	if err != nil {
		fmt.Fprintln(errOut, "meno:", err)
		return 1
	}
	defer h.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	res, err := headlessRun(ctx, h.client, l, prompt, errOut)
	if l.json {
		b, _ := json.Marshal(res)
		fmt.Fprintln(out, string(b))
	} else if res.Text != "" {
		fmt.Fprintln(out, strings.TrimRight(res.Text, "\n"))
	}
	if err != nil {
		fmt.Fprintln(errOut, "meno:", err)
		return 1
	}
	return 0
}

func readPrompt(l launch, stdin io.Reader) (string, error) {
	prompt := l.prompt
	if !l.hasInput {
		if f, ok := stdin.(*os.File); ok {
			if st, err := f.Stat(); err == nil && st.Mode()&os.ModeCharDevice != 0 {
				return "", errors.New("-p needs a prompt: meno -p \"...\" or pipe it on stdin")
			}
		}
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", err
		}
		prompt = string(b)
	}
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("the prompt is empty")
	}
	return prompt, nil
}

// headlessRun opens or creates the session, sends the prompt and follows the
// run to its end. A question of the agent is an error: nobody can answer it.
func headlessRun(ctx context.Context, c *client.Client, l launch, prompt string, progress io.Writer) (headlessResult, error) {
	var res headlessResult
	cwd := l.cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	id, created, err := openForRun(ctx, c, l, cwd)
	if err != nil {
		return res, err
	}
	res.SessionID, res.Status = id, "failed"
	if created && l.noSave {
		defer func() { _ = c.DeleteSession(context.Background(), id) }()
	}
	snap, err := c.Snapshot(ctx, id)
	if err != nil {
		return res, err
	}
	if !created && l.model != "" {
		if _, err = c.PatchSession(ctx, id, api.PatchSessionRequest{Select: &api.ModelSelection{Model: l.model}}); err != nil {
			return res, err
		}
	}
	if _, err = c.Message(ctx, id, api.MessageRequest{Text: prompt, ClientID: newClientID(), Client: "cli"}); err != nil {
		return res, err
	}
	text, err := followRun(ctx, c, id, snap.Seq, progress)
	res.Text = text
	switch {
	case err == nil:
		res.Status = "done"
	case ctx.Err() != nil:
		res.Status, res.Error = "interrupted", "interrupted"
		_ = c.Stop(context.Background(), id, api.StopRequest{})
		return res, errors.New("interrupted")
	default:
		res.Error = err.Error()
	}
	return res, err
}

func openForRun(ctx context.Context, c *client.Client, l launch, cwd string) (id string, created bool, err error) {
	switch {
	case l.resume != "":
		return l.resume, false, nil
	case l.cont:
		id, err = newestSession(ctx, c, cwd)
		return id, false, err
	}
	r, err := c.CreateSession(ctx, api.SessionCreateRequest{Cwd: cwd, ClientID: newClientID(), NoAsk: l.noAsk || noTTY(), Model: l.model})
	if err != nil {
		return "", false, err
	}
	return r.Session.ID, true, nil
}

func noTTY() bool {
	st, err := os.Stdin.Stat()
	return err != nil || st.Mode()&os.ModeCharDevice == 0
}

// newestSession is the most recently used stored session of cwd.
func newestSession(ctx context.Context, c *client.Client, cwd string) (string, error) {
	list, err := c.ListSessions(ctx, api.SessionsListRequest{Cwd: cwd, Limit: 1})
	if err != nil {
		return "", err
	}
	if len(list.Sessions) == 0 {
		return "", fmt.Errorf("no session to continue in %s", cwd)
	}
	return list.Sessions[0].ID, nil
}

// followRun reads events until the run ends and returns what the agent said.
func followRun(ctx context.Context, c *client.Client, id string, after uint64, progress io.Writer) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var text string
	var failure error
	finished := false
	end := func() error { finished = true; cancel(); return errStopStream }
	err := c.Stream(ctx, id, after, "", func(e api.Event) error {
		switch e.Type {
		case api.EventTextDelta:
			var d api.TextDeltaData
			if json.Unmarshal(e.Data, &d) == nil {
				text += d.Text
			}
		case api.EventCallStart:
			var d api.CallStartData
			if json.Unmarshal(e.Data, &d) == nil && progress != nil {
				fmt.Fprintln(progress, "$", d.Command)
			}
		case api.EventTurnAdded:
			var d api.TurnAddedData
			if json.Unmarshal(e.Data, &d) == nil && d.Turn.Role == "assistant" {
				text = d.Turn.Content // empty for a tool-call turn: the next answer starts fresh
			}
		case api.EventAskOpen:
			failure = errors.New("the agent asked a question and no one can answer (use --no-ask)")
			return end()
		case api.EventError:
			var d api.ErrorData
			_ = json.Unmarshal(e.Data, &d)
			failure = errors.New(d.Message)
			return end()
		case api.EventAborted:
			failure = errors.New("the run was stopped")
			return end()
		case api.EventDone:
			return end()
		}
		return nil
	})
	switch {
	case failure != nil:
		return text, failure
	case err != nil && !finished:
		return text, err
	case !finished:
		return text, errors.New("the event stream ended before the run did")
	}
	return text, nil
}
