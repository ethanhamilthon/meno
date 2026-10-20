package cli

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

func runAsk(args []string, s IO, a API, j bool) (int, error) {
	if len(args) != 1 {
		return 0, usageError("usage: meno ask FILE|-")
	}
	if err := apiRequired(a); err != nil {
		return 0, err
	}
	var b []byte
	var err error
	if args[0] == "-" {
		b, err = io.ReadAll(s.In)
	} else {
		b, err = os.ReadFile(cleanPath(args[0], s))
	}
	if err != nil {
		return 0, err
	}
	var form any
	if err := json.Unmarshal(b, &form); err != nil {
		return 0, usageError("invalid form JSON: " + err.Error())
	}
	answer, err := a.Ask(b)
	if err != nil {
		return 0, err
	}
	return 0, emit(s, true, answer, "")
}
func runGoal(args []string, s IO, a API, j bool) (int, error) {
	if len(args) < 2 {
		return 0, usageError("usage: meno goal done SUMMARY|blocked REASON")
	}
	if err := apiRequired(a); err != nil {
		return 0, err
	}
	text := strings.Join(args[1:], " ")
	var err error
	switch args[0] {
	case "done":
		err = a.GoalDone(text)
	case "blocked":
		err = a.GoalBlocked(text)
	default:
		return 0, unknown(args[0], []string{"done", "blocked"})
	}
	if err != nil {
		return 0, err
	}
	return 0, emit(s, j, map[string]string{"status": args[0], "text": text}, args[0]+" sent\n")
}
func runView(args []string, s IO, a API, j bool) (int, error) {
	if len(args) != 1 {
		return 0, usageError("usage: meno view IMAGE")
	}
	if err := apiRequired(a); err != nil {
		return 0, err
	}
	res, err := a.View(args[0])
	if err != nil {
		return 0, err
	}
	plain := "image displayed\n"
	if !res.Displayed {
		plain = "image queued\n"
	}
	return 0, emit(s, j, res, plain)
}
func runModels(args []string, s IO, a API, j bool) (int, error) {
	values, switches, pos, err := parseFlags(args, map[string]bool{"provider": true}, map[string]bool{"refresh": true})
	if err != nil {
		return 0, err
	}
	if len(pos) > 0 {
		return 0, usageError("usage: meno models [--provider NAME --refresh]")
	}
	if err = apiRequired(a); err != nil {
		return 0, err
	}
	items, err := a.Models(values["provider"], switches["refresh"])
	if err != nil {
		return 0, err
	}
	return 0, emit(s, j, items, formatList(items, func(x Model) string {
		if x.Name != "" {
			return x.Provider + "/" + x.ID + "\t" + x.Name
		}
		return x.Provider + "/" + x.ID
	}))
}
