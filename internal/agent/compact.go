package agent

import (
	"context"
	"strings"

	"meno/internal/llm"
)

const keepTail = 4

func Compact(ctx context.Context, cl llm.Client, p llm.Provider, model string, s *Session) (before, after int, err error) {
	all := s.Snapshot()
	before = EstimateTokens(s)
	if len(all) <= keepTail {
		return before, before, nil
	}
	head := all[:len(all)-keepTail]
	tail := all[len(all)-keepTail:]
	var b strings.Builder
	for _, t := range head {
		b.WriteString(t.Role + ": " + t.Text)
		if t.Code != "" {
			b.WriteString("\n[js] " + t.Code + "\n[out] " + t.Output)
		}
		b.WriteString("\n")
	}
	prompt := "Summarize this conversation for continuation. Keep decisions, file paths, results. Be concise.\n\n" + b.String()
	var sum strings.Builder
	err = cl.Stream(ctx, llm.Request{Provider: p, Model: model, Msgs: []llm.Msg{{Role: llm.RoleUser, Text: prompt}}}, func(e llm.Event) {
		if e.Kind == llm.EvText {
			sum.WriteString(e.Text)
		}
	})
	if err != nil {
		return before, before, err
	}
	s.Summary = strings.TrimSpace(sum.String())
	s.Replace(tail)
	after = EstimateTokens(s)
	return before, after, nil
}

func EstimateTokens(s *Session) int {
	n := len(s.Summary)
	for _, t := range s.Snapshot() {
		n += len(t.Text) + len(t.Code) + len(t.Output) + len(t.Err)
	}
	return n / 4
}
