package update

import (
	"context"
	"path/filepath"
	"time"

	"meno/internal/store"
)

// CheckEvery is how long a check result stays fresh.
const CheckEvery = 6 * time.Hour

type cache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

// Check returns the newest release tag when it is newer than current, or "".
// The answer is cached in home/update.json for CheckEvery so that starting
// meno does not ask GitHub every time. Errors count as "no update".
func (c Client) Check(ctx context.Context, home, current string, now time.Time) string {
	path := filepath.Join(home, "update.json")
	var ca cache
	_ = store.LoadJSON(path, &ca)
	if ca.Latest == "" || now.Sub(ca.CheckedAt) >= CheckEvery || ca.CheckedAt.After(now) {
		tag, err := c.Latest(ctx)
		if err != nil {
			return ""
		}
		ca = cache{CheckedAt: now, Latest: tag}
		_ = store.SaveJSON(path, ca)
	}
	if Newer(current, ca.Latest) {
		return ca.Latest
	}
	return ""
}
