package cli

import (
	"fmt"
	"strings"
)

// Exit codes of `meno doctor`: 0 healthy, 4 warnings only, 1 at least one error.
const (
	doctorWarnExit = 4
	doctorFailExit = 1
)

func runDoctor(args []string, s IO, a API, j bool) (int, error) {
	_, switches, pos, err := parseFlags(args, nil, map[string]bool{"probe": true})
	if err != nil {
		return 0, err
	}
	if len(pos) > 0 {
		return 0, usageError("usage: meno doctor [--probe] [--json]")
	}
	if err = apiRequired(a); err != nil {
		return 0, err
	}
	rep, err := a.Doctor(DoctorRequest{Probe: switches["probe"]})
	if err != nil {
		return 0, err
	}
	rep.Status = worst(rep.Checks)
	if err = emit(s, j, rep, doctorText(rep)); err != nil {
		return 0, err
	}
	switch rep.Status {
	case "error":
		return doctorFailExit, nil
	case "warn":
		return doctorWarnExit, nil
	}
	return 0, nil
}

func worst(checks []Check) string {
	out := "ok"
	for _, c := range checks {
		if c.Status == "error" {
			return "error"
		}
		if c.Status == "warn" {
			out = "warn"
		}
	}
	return out
}

func doctorText(r DoctorReport) string {
	var b strings.Builder
	for _, c := range r.Checks {
		fmt.Fprintf(&b, "%-5s %s", c.Status, c.Name)
		if c.Detail != "" {
			b.WriteString(": " + c.Detail)
		}
		b.WriteString("\n")
		if c.Fix != "" && c.Status != "ok" {
			b.WriteString("      fix: " + c.Fix + "\n")
		}
	}
	b.WriteString("result: " + r.Status + "\n")
	return b.String()
}
