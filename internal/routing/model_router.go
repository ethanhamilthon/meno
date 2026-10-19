package routing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"meno/internal/llm"
)

const DefaultTimeout = 5 * time.Second

const routerInstructions = "Choose the reasoning effort for this task.\nLow is for a mechanical, straightforward change. Medium is for normal work. High is for a task that needs careful reasoning and can fail if rushed.\nUse only one of the allowed levels, or keep the current level. Return exactly two lines, with no extra text:\nlevel: <allowed level or keep>\nreason: <one short sentence>"

// ModelRouter asks a provider model to choose one allowed effort level.
type ModelRouter struct {
	Client   llm.Client
	Provider llm.Provider
	Model    string
	Timeout  time.Duration
}

// Route sends one small, tool-free request to the configured router model.
func (m ModelRouter) Route(ctx context.Context, r Request) (Verdict, error) {
	if m.Client == nil {
		return Verdict{}, fmt.Errorf("effort router client is not configured")
	}
	timeout := m.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	levels := m.Provider.EffortsFor(m.Model, []string{"low", "medium", "high", "max"})
	if len(levels) == 0 {
		return Verdict{}, fmt.Errorf("effort router model has no supported effort levels")
	}
	var answer strings.Builder
	var streamErr error
	err := m.Client.Stream(ctx, llm.Request{
		Provider: m.Provider,
		Model:    m.Model,
		Effort:   levels[0],
		System:   routerSystemPrompt(r.Allowed),
		Msgs:     []llm.Msg{{Role: llm.RoleUser, Text: contextMessage(r)}},
	}, func(ev llm.Event) {
		switch ev.Kind {
		case llm.EvText:
			answer.WriteString(ev.Text)
		case llm.EvError:
			if ev.Err != nil {
				streamErr = ev.Err
			} else {
				streamErr = fmt.Errorf("effort router stream failed")
			}
		}
	})
	if err != nil {
		return Verdict{}, err
	}
	if streamErr != nil {
		return Verdict{}, streamErr
	}
	return parseVerdict(answer.String(), r.Allowed)
}

func routerSystemPrompt(allowed []string) string {
	return routerInstructions + "\nAllowed levels: " + strings.Join(allowed, ", ") + "."
}

func contextMessage(r Request) string {
	modes := strings.Join(r.Context.Modes, ", ")
	if modes == "" {
		modes = "none"
	}
	return fmt.Sprintf("Current effort: %s\nModes: %s\nAttachments: %d\nProject entries: %d\n\nTask:\n%s",
		r.Current, modes, r.Context.Attachments, r.Context.Files, r.Context.Text)
}

func parseVerdict(answer string, allowed []string) (Verdict, error) {
	lines := strings.Split(strings.TrimSpace(answer), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "level: ") || !strings.HasPrefix(lines[1], "reason: ") {
		return Verdict{}, fmt.Errorf("effort router returned an invalid answer")
	}
	level := strings.TrimSpace(strings.TrimPrefix(lines[0], "level: "))
	why := strings.TrimSpace(strings.TrimPrefix(lines[1], "reason: "))
	if lines[0] != "level: "+level || lines[1] != "reason: "+why {
		return Verdict{}, fmt.Errorf("effort router returned an invalid answer")
	}
	if why == "" {
		return Verdict{}, fmt.Errorf("effort router returned no reason")
	}
	if level == "keep" {
		return Verdict{Why: why}, nil
	}
	for _, option := range allowed {
		if level == option {
			return Verdict{Level: level, Why: why}, nil
		}
	}
	return Verdict{}, fmt.Errorf("effort router returned unsupported level %q", level)
}
