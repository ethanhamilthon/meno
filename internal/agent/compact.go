package agent

import (
	"context"
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
	// The request the session is sent with. When System is set, the summary is
	// asked for with that system prompt, the tool schema and the messages of
	// the part being summarized, so the provider's prefix cache serves almost
	// all of it. If that fails, the transcript is sent as a single message.
	System   string
	Effort   string
	ToolDesc string
	ToolName string
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
	instruction := summaryInstruction(o.SummaryTokens)
	var summary string
	if o.System != "" {
		req := llm.Request{Provider: p, Model: model, Effort: o.Effort, System: o.System, Tools: true, ToolDescription: o.ToolDesc, ToolName: o.ToolName,
			Msgs: append(s.msgsOf(all[:cut]), llm.Msg{Role: llm.RoleUser, Text: instruction + "\nThe last " + strconv.Itoa(len(tail)) + " messages stay as they are and are not part of this summary. Answer with the summary as text; do not call any tool."})}
		summary, err = askSummary(ctx, cl, req, reportUsage)
		if err != nil && ctx.Err() != nil {
			return before, before, ctx.Err()
		}
	}
	if o.System == "" || err != nil {
		req := llm.Request{Provider: p, Model: model, Msgs: []llm.Msg{{Role: llm.RoleUser, Text: instruction + "\n" + b.String()}}}
		summary, err = askSummary(ctx, cl, req, reportUsage)
	}
	if err != nil {
		return before, before, err
	}
	s.Summary = summary
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
