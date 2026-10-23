package cli

import (
	"fmt"
	"strings"
	"time"
)

const sendUsage = "usage: meno send (-s ID | --new | --fork SESSION) [--at N] [--model PROVIDER/MODEL] [--effort E] [--cwd DIR] [--title T] [--system NAME] [--no-ask] [--detach | --timeout SECONDS] [--keep-running | --cancel-on-exit] TEXT"

// parseSend turns the arguments of `meno send` into a request. Exactly one of
// -s, --new and --fork says where to write; there is no implicit session.
func parseSend(args []string) (SendRequest, error) {
	values, sw, pos, err := parseFlags(args,
		map[string]bool{"s": true, "fork": true, "at": true, "model": true, "effort": true, "cwd": true, "title": true, "system": true, "timeout": true},
		map[string]bool{"new": true, "detach": true, "no-ask": true, "keep-running": true, "cancel-on-exit": true})
	if err != nil {
		return SendRequest{}, err
	}
	if len(pos) == 0 {
		return SendRequest{}, usageError(sendUsage)
	}
	where := 0
	for _, set := range []bool{values["s"] != "", sw["new"], values["fork"] != ""} {
		if set {
			where++
		}
	}
	switch {
	case where == 0:
		return SendRequest{}, usageError("choose where to write: -s ID for an existing session, --new, or --fork SESSION\n" + sendUsage)
	case where > 1:
		return SendRequest{}, usageError("-s, --new and --fork cannot be combined")
	}
	fresh := sw["new"] || values["fork"] != ""
	if !fresh {
		for _, f := range []string{"cwd", "title", "system", "at"} {
			if values[f] != "" {
				return SendRequest{}, usageError("--" + f + " applies to a new session or a fork: use it with --new or --fork")
			}
		}
		if sw["no-ask"] {
			return SendRequest{}, usageError("--no-ask applies to a new session: use it with --new or --fork")
		}
	}
	if values["at"] != "" && values["fork"] == "" {
		return SendRequest{}, usageError("--at applies to --fork")
	}
	if sw["keep-running"] && sw["cancel-on-exit"] {
		return SendRequest{}, usageError("--keep-running and --cancel-on-exit cannot be combined")
	}
	at, err := intFlag("at", values["at"], 0, 1)
	if err != nil {
		return SendRequest{}, err
	}
	secs, err := intFlag("timeout", values["timeout"], 0, 1)
	if err != nil {
		return SendRequest{}, err
	}
	if secs > 0 && sw["detach"] {
		return SendRequest{}, usageError("--timeout waits for the run; it cannot be combined with --detach")
	}
	return SendRequest{
		Text: strings.Join(pos, " "), SessionID: values["s"], New: sw["new"], Fork: values["fork"], At: at,
		NoAsk: sw["no-ask"], Detach: sw["detach"], Model: values["model"], Effort: values["effort"],
		CWD: values["cwd"], Title: values["title"], System: values["system"],
		Timeout: time.Duration(secs) * time.Second, KeepRunning: sw["keep-running"], CancelOnExit: sw["cancel-on-exit"],
	}, nil
}

func runSend(args []string, s IO, a API, j bool) (int, error) {
	req, err := parseSend(args)
	if err != nil {
		return 0, err
	}
	if err = apiRequired(a); err != nil {
		return 0, err
	}
	res, err := a.Send(req)
	for _, w := range res.Warnings {
		fmt.Fprintf(s.Err, "meno: warning: %s\n", w)
	}
	if err != nil {
		if j && res.SessionID != "" {
			_ = emit(s, true, res, "")
		}
		return 0, err
	}
	plain := res.Text
	switch {
	case res.Queued:
		fmt.Fprintf(s.Err, "meno: session %s is busy: the message is queued behind the running turn\n", res.SessionID)
	case res.Status == "detached" && (req.New || req.Fork != ""):
		plain = res.SessionID + "\n"
	case res.Status == "waiting":
		q := res.Question
		fmt.Fprintf(s.Err, "meno: session %s waits for an answer to question %s; answer with `meno sessions answer %s %s '{\"%s\": ...}'` or `--cancel`\n",
			res.SessionID, q.ID, res.SessionID, q.ID, firstQuestionID(q))
		if err = emit(s, j, res, plain); err != nil {
			return 0, err
		}
		return ExitNeedsAnswer, nil
	}
	return 0, emit(s, j, res, plain)
}

func firstQuestionID(q *PendingQuestion) string {
	if q != nil && len(q.Questions) > 0 {
		return q.Questions[0].ID
	}
	return "q1"
}
