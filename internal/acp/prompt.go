package acp

import (
	"encoding/json"
	"strings"

	"meno/internal/api"
	"meno/internal/content"
)

type promptParams struct {
	SessionID string  `json:"sessionId"`
	Prompt    []block `json:"prompt"`
}

func stopResult(reason string) map[string]any { return map[string]any{"stopReason": reason} }

// prompt serves session/prompt: commands run locally, anything else is a user
// message to the agent.
func (s *Server) prompt(raw json.RawMessage) (any, error) {
	var p promptParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errParams("invalid params: %v", err)
	}
	x, err := s.session(p.SessionID)
	if err != nil {
		return nil, err
	}
	in, err := parsePrompt(p.Prompt)
	if err != nil {
		return nil, err
	}
	if name, args, ok := parseCommand(in.Text); ok && len(in.Attachments) == 0 {
		return s.command(x, name, args)
	}
	x.mu.Lock()
	open := x.pendingAsk
	x.mu.Unlock()
	if open != nil {
		x.say("A question is open (id `" + open.AskID + "`). Answer it with `/answer " + open.AskID + " <answers-json>` or cancel it with `/answer " + open.AskID + " --cancel`; the agent is waiting.")
		return stopResult("end_turn"), nil
	}
	return s.run(x, func() error {
		res, err := s.backend.Message(s.uncancelled(), x.id, api.MessageRequest{Text: in.Text, Attachments: in.Attachments, ClientID: clientID(), Origin: "acp"})
		if err == nil {
			x.started(res.RequestID)
		}
		return err
	})
}

// run registers the foreground prompt, starts it and waits for its outcome.
func (s *Server) run(x *session, start func() error) (any, error) {
	w, err := x.begin()
	if err != nil {
		return nil, err
	}
	if err := start(); err != nil {
		x.abandon()
		return nil, err
	}
	o := x.wait(s.ctx, w)
	if o.err != nil {
		return nil, o.err
	}
	return stopResult(o.stop), nil
}

func (x *session) say(text string) { x.send(chunk("agent_message_chunk", text)) }

// parseCommand recognises "/name args". A path such as /usr/bin is not a command.
func parseCommand(text string) (name, args string, ok bool) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "/") || strings.Contains(t, "\n") && !strings.HasPrefix(t, "/answer") {
		return "", "", false
	}
	t = t[1:]
	name = t
	if i := strings.IndexAny(t, " \t\n"); i >= 0 {
		name, args = t[:i], strings.TrimSpace(t[i+1:])
	}
	if name == "" || strings.ContainsAny(name, "/\\.") {
		return "", "", false
	}
	return name, args, true
}

func (s *Server) command(x *session, name, args string) (any, error) {
	switch {
	case name == "answer":
		return s.answer(x, args)
	case name == "compact":
		res, err := s.backend.Compact(s.ctx, x.id, api.CompactRequest{ClientID: clientID()})
		if err != nil {
			x.say("Compaction failed: " + err.Error())
		} else {
			x.say("Compacted the conversation: removed " + itoa(res.Removed) + " messages.")
		}
	case name == "rename":
		if args == "" {
			x.say("Usage: /rename <title>")
			break
		}
		if _, err := s.backend.PatchSession(s.ctx, x.id, api.PatchSessionRequest{Name: &args}); err != nil {
			x.say("Rename failed: " + err.Error())
		} else {
			x.say("Renamed to " + args + ".")
		}
	case name == "skills":
		x.say(s.skillsText())
	case name == "context":
		x.say(s.contextText(x))
	case name == "help":
		x.say(helpText)
	case strings.HasPrefix(name, "skill:"):
		skill := strings.TrimPrefix(name, "skill:")
		text := content.Object{Kind: content.Skill, Ref: skill}.Tag()
		if args != "" {
			text += " " + args
		}
		x.mu.Lock()
		open := x.pendingAsk
		x.mu.Unlock()
		if open != nil {
			x.say("A question is open (id `" + open.AskID + "`); answer it first.")
			break
		}
		return s.run(x, func() error {
			res, err := s.backend.Message(s.uncancelled(), x.id, api.MessageRequest{Text: text, ClientID: clientID(), Origin: "acp"})
			if err == nil {
				x.started(res.RequestID)
			}
			return err
		})
	default:
		x.say("Unknown command /" + name + ". Type /help for the list.")
	}
	return stopResult("end_turn"), nil
}

const helpText = "Commands: /answer <id> <answers-json> | /answer <id> --cancel, /compact, /rename <title>, /skills, /skill:<name> [text], /context, /help."

// answer submits a form answer and resumes the paused run.
func (s *Server) answer(x *session, args string) (any, error) {
	x.mu.Lock()
	open := x.pendingAsk
	x.mu.Unlock()
	id, rest, _ := strings.Cut(args, " ")
	rest = strings.TrimSpace(rest)
	if open == nil || id == "" || id != open.AskID {
		if open == nil {
			x.say("There is no open question.")
		} else {
			x.say("The open question has id `" + open.AskID + "`. Usage: /answer " + open.AskID + " <answers-json>")
		}
		return stopResult("end_turn"), nil
	}
	req := api.AskAnswerRequest{}
	if rest == "--cancel" {
		req.Cancel = true
	} else if err := json.Unmarshal([]byte(rest), &req.Answers); err != nil || len(req.Answers) == 0 {
		x.say("The answers must be a JSON object keyed by question id, for example: " + string(mustJSON(map[string]string{open.Questions[0].ID: "..."})) + ". The question stays open.")
		return stopResult("end_turn"), nil
	}
	w, err := x.begin()
	if err != nil {
		return nil, err
	}
	if err := s.backend.AnswerAsk(s.ctx, x.id, id, req); err != nil {
		x.abandon()
		x.say("The answer was not accepted: " + err.Error() + ". The question stays open.")
		return stopResult("end_turn"), nil
	}
	x.mu.Lock()
	rid := x.requestID
	x.mu.Unlock()
	x.started(rid)
	o := x.wait(s.ctx, w)
	if o.err != nil {
		return nil, o.err
	}
	return stopResult(o.stop), nil
}
