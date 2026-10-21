package agent

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"meno/internal/llm"
)

// summaryInstruction is what the model is asked to write. It starts the same
// way on both paths of a compaction.
func summaryInstruction(tokens int) string {
	s := "Summarize this conversation for continuation as Markdown with these sections: goal and acceptance criteria; current and superseded decisions; completed and remaining work; constraints and exclusions; changed files and observed Git state; validation done and what is unverified; artifact and result IDs; next action and blockers. Keep decisions, file paths and results. Be concise.\n"
	if tokens > 0 {
		s += "Keep the summary under about " + strconv.Itoa(tokens) + " tokens.\n"
	}
	return s
}

// askSummary sends one summary request and returns the trimmed text. A
// request the model answered with a tool call or with nothing is an error, so
// the caller can try the other way.
func askSummary(ctx context.Context, cl llm.Client, req llm.Request, reportUsage []func(llm.Usage), recordUsage func(llm.Usage)) (string, error) {
	var sum strings.Builder
	var streamErr error
	called := false
	err := cl.Stream(ctx, req, func(e llm.Event) {
		switch e.Kind {
		case llm.EvError:
			streamErr = errors.New(e.Text)
		case llm.EvDone:
			if recordUsage != nil && (e.UsageReported || e.Usage != (llm.Usage{})) {
				recordUsage(e.Usage)
			}
			for _, report := range reportUsage {
				if report != nil {
					report(e.Usage)
				}
			}
		case llm.EvText:
			sum.WriteString(e.Text)
		case llm.EvTool:
			called = true
		}
	})
	if err == nil {
		err = streamErr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && called {
		err = errors.New("the model called a tool instead of writing the summary")
	}
	if err == nil && strings.TrimSpace(sum.String()) == "" {
		err = errors.New("empty summary; context unchanged")
	}
	return strings.TrimSpace(sum.String()), err
}
