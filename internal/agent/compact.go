package agent

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"meno/internal/llm"
)

const keepTail = 4

// CompactOptions tune a summary-generating compaction.
type CompactOptions struct {
	Keep          int // recent messages that stay in the active context (default 4)
	SummaryTokens int // target size of the summary (0 = unbounded)
}

// Compact summarizes the old part of the active context with the model and
// moves it to the history; nothing is deleted.
func Compact(ctx context.Context, cl llm.Client, p llm.Provider, model string, s *Session, reportUsage ...func(llm.Usage)) (before, after int, err error) {
	return CompactWith(ctx, cl, p, model, s, CompactOptions{}, reportUsage...)
}

// CompactWith is Compact with options.
func CompactWith(ctx context.Context, cl llm.Client, p llm.Provider, model string, s *Session, o CompactOptions, reportUsage ...func(llm.Usage)) (before, after int, err error) {
	keep := o.Keep
	if keep <= 0 {
		keep = keepTail
	}
	all := s.Snapshot()
	before = EstimateTokens(s)
	if len(all) <= keep {
		return before, before, nil
	}
	cut := len(all) - keep
	// Never split the assistant output from any of its tool results.
	for cut > 0 && all[cut].ReplayGroup != "" && all[cut-1].ReplayGroup == all[cut].ReplayGroup {
		cut--
	}
	if cut == 0 {
		return before, before, nil
	}
	head := all[:cut]
	tail := all[cut:]
	var b strings.Builder
	if s.Summary != "" {
		b.WriteString("Previous summary:\n" + s.Summary + "\n")
	}
	for _, t := range head {
		b.WriteString(t.Role + ": " + s.TextWithImagePaths(t, func(path string) string { return "[image: " + path + "]" }))
		if t.Code != "" || t.CallID != "" {
			b.WriteString("\n[js] " + t.Code + "\n[out] " + t.Output)
		}
		if t.Err != "" {
			b.WriteString("\nERROR: " + t.Err)
		}
		b.WriteString("\n")
	}
	prompt := "Summarize this conversation for continuation as Markdown with these sections: goal and acceptance criteria; current and superseded decisions; completed and remaining work; constraints and exclusions; changed files and observed Git state; validation done and what is unverified; artifact and result IDs; next action and blockers. Keep decisions, file paths and results. Be concise.\n"
	if o.SummaryTokens > 0 {
		prompt += "Keep the summary under about " + strconv.Itoa(o.SummaryTokens) + " tokens.\n"
	}
	prompt += "\n" + b.String()
	var sum strings.Builder
	var streamErr error
	err = cl.Stream(ctx, llm.Request{Provider: p, Model: model, Msgs: []llm.Msg{{Role: llm.RoleUser, Text: prompt}}}, func(e llm.Event) {
		if e.Kind == llm.EvError {
			streamErr = errors.New(e.Text)
		}
		if e.Kind == llm.EvDone {
			for _, report := range reportUsage {
				if report != nil {
					report(e.Usage)
				}
			}
		}
		if e.Kind == llm.EvText {
			sum.WriteString(e.Text)
		}
	})
	if err == nil {
		err = streamErr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && strings.TrimSpace(sum.String()) == "" {
		err = errors.New("empty summary; context unchanged")
	}
	if err != nil {
		return before, before, err
	}
	s.Summary = strings.TrimSpace(sum.String())
	s.ensureLog().Compact(head, tail, SummaryRev{Rev: int(time.Now().UnixNano() / 1000), Summary: s.Summary, Time: time.Now()})
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
