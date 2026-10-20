package agent

import (
	"context"

	"meno/internal/db"
	"meno/internal/llm"
)

// Turn appends a user message and runs the agent; only tests drive a run this way.
func (r *Runner) Turn(ctx context.Context, s *Session, userText string) error {
	s.Append(Turn{Role: llm.RoleUser, Text: userText})
	return r.Continue(ctx, s)
}

// CloseSessionStores closes every opened database (flushing the WAL).
func CloseSessionStores() { db.CloseAll() }

// Compact summarizes with default options.
func Compact(ctx context.Context, cl llm.Client, p llm.Provider, model string, s *Session, reportUsage ...func(llm.Usage)) (before, after int, err error) {
	return CompactWith(ctx, cl, p, model, s, CompactOptions{}, reportUsage...)
}

// ReadHistory returns turns of the stored history of a session after a history ID.
func ReadHistory(dir, id string, after int64, limit int) ([]Turn, error) {
	d, err := openDB(dir)
	if err != nil {
		return nil, err
	}
	return readHistory(d, id, after, limit)
}
