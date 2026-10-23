package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

var sessionSubcommands = []string{"show", "cancel", "answer"}

func runSessions(args []string, s IO, a API, j bool) (int, error) {
	if len(args) > 0 && contains(sessionSubcommands, args[0]) {
		if err := apiRequired(a); err != nil {
			return 0, err
		}
		switch args[0] {
		case "show":
			return runSessionShow(args[1:], s, a, j)
		case "cancel":
			return runSessionCancel(args[1:], s, a, j)
		default:
			return runSessionAnswer(args[1:], s, a, j)
		}
	}
	values, _, pos, err := parseFlags(args, map[string]bool{"cwd": true, "status": true, "spawned-by": true, "title-prefix": true, "limit": true}, nil)
	if err != nil {
		return 0, err
	}
	if len(pos) > 0 {
		return 0, usageError("usage: meno sessions [--cwd DIR] [--status idle|running|needs-you] [--spawned-by ID|me] [--title-prefix P] [--limit N]\n       meno sessions show ID | cancel ID | answer ID QUESTION ANSWERS_JSON|--cancel")
	}
	limit, err := intFlag("limit", values["limit"], 0, 1)
	if err != nil {
		return 0, err
	}
	if err = apiRequired(a); err != nil {
		return 0, err
	}
	items, err := a.Sessions(SessionsRequest{CWD: values["cwd"], Status: values["status"], SpawnedBy: values["spawned-by"], TitlePrefix: values["title-prefix"], Limit: limit})
	if err != nil {
		return 0, err
	}
	return 0, emit(s, j, items, formatList(items, func(x Session) string { return x.ID + "\t" + x.Status + "\t" + x.Name + "\t" + x.CWD }))
}

func runSessionShow(args []string, s IO, a API, j bool) (int, error) {
	if len(args) != 1 {
		return 0, usageError("usage: meno sessions show ID")
	}
	d, err := a.SessionShow(args[0])
	if err != nil {
		return 0, err
	}
	return 0, emit(s, j, d, detailText(d))
}

func detailText(d SessionDetail) string {
	var b strings.Builder
	line := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	line("session", d.ID)
	line("name", d.Name)
	line("status", d.Status)
	if d.Model != "" {
		m := d.Provider + "/" + d.Model
		if d.Effort != "" {
			m += " (effort " + d.Effort + ")"
		}
		line("model", m)
	}
	line("cwd", d.CWD)
	fmt.Fprintf(&b, "turns: %d\n", d.Turns)
	line("spawned by", d.SpawnedBy)
	line("fork of", d.ForkOf)
	line("system prompt", d.System)
	if d.Tasks > 0 {
		fmt.Fprintf(&b, "tasks: %d\n", d.Tasks)
	}
	if d.Usage != nil {
		fmt.Fprintf(&b, "tokens: %d in, %d out, cost %.4f\n", d.Usage.InputTokens, d.Usage.OutputTokens, d.Usage.Cost)
	}
	line("last error", d.LastError)
	if q := d.Question; q != nil {
		fmt.Fprintf(&b, "question %s:\n", q.ID)
		for _, x := range q.Questions {
			fmt.Fprintf(&b, "  %s (%s): %s", x.ID, x.Type, x.Prompt)
			if len(x.Options) > 0 {
				fmt.Fprintf(&b, " [%s]", strings.Join(x.Options, ", "))
			}
			b.WriteByte('\n')
		}
	}
	if d.Answer != "" {
		b.WriteString("answer:\n" + d.Answer + "\n")
	}
	return b.String()
}

func runSessionCancel(args []string, s IO, a API, j bool) (int, error) {
	if len(args) != 1 {
		return 0, usageError("usage: meno sessions cancel ID")
	}
	if err := a.SessionCancel(args[0]); err != nil {
		return 0, err
	}
	return 0, emit(s, j, map[string]any{"session": args[0], "stopped": true}, "stop requested for "+args[0]+"\n")
}

func runSessionAnswer(args []string, s IO, a API, j bool) (int, error) {
	const usage = "usage: meno sessions answer ID QUESTION ANSWERS_JSON | --cancel"
	_, sw, pos, err := parseFlags(args, nil, map[string]bool{"cancel": true})
	if err != nil {
		return 0, err
	}
	want := 3
	if sw["cancel"] {
		want = 2
	}
	if len(pos) != want {
		return 0, usageError(usage)
	}
	req := AnswerRequest{SessionID: pos[0], QuestionID: pos[1], Cancel: sw["cancel"]}
	if !sw["cancel"] {
		if err = json.Unmarshal([]byte(pos[2]), &req.Answers); err != nil || len(req.Answers) == 0 {
			return 0, usageError(`the answers must be a JSON object keyed by question id, for example {"q1": "yes"}`)
		}
	}
	if err = a.SessionAnswer(req); err != nil {
		return 0, err
	}
	return 0, emit(s, j, map[string]any{"session": req.SessionID, "question": req.QuestionID, "cancelled": req.Cancel}, "answered "+req.QuestionID+"\n")
}
