package acp

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"

	"meno/internal/api"
)

// List pages a stable ordering and binds cursors to the directory filter. The
// cursor is an offset, not a promise of snapshot isolation across list changes.
func (s *Server) listSessions(raw json.RawMessage) (any, error) {
	var p struct {
		Cwd    string `json:"cwd"`
		Cursor string `json:"cursor"`
	}
	if len(raw) > 0 && (string(raw) == "null" || json.Unmarshal(raw, &p) != nil) {
		return nil, errParams("invalid list params")
	}
	if p.Cwd != "" && !filepath.IsAbs(p.Cwd) {
		return nil, errParams("cwd must be absolute")
	}
	offset := 0
	if p.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(p.Cursor)
		var cursor struct {
			Offset int
			Cwd    string
		}
		if err != nil || json.Unmarshal(decoded, &cursor) != nil || cursor.Offset < 0 || cursor.Cwd != p.Cwd {
			return nil, errParams("invalid list cursor")
		}
		offset = cursor.Offset
	}
	res, err := s.backend.ListSessions(s.ctx, api.SessionsListRequest{Cwd: p.Cwd})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(res.Sessions, func(i, j int) bool {
		a, b := res.Sessions[i], res.Sessions[j]
		if a.UpdatedAt != b.UpdatedAt {
			return a.UpdatedAt > b.UpdatedAt
		}
		return a.ID < b.ID
	})
	items := make([]map[string]any, 0)
	end := min(len(res.Sessions), offset+200)
	if offset > len(res.Sessions) {
		return nil, errParams("list cursor exceeds available sessions")
	}
	for _, it := range res.Sessions[offset:end] {
		m := map[string]any{"sessionId": it.ID, "cwd": it.Cwd}
		if it.Name != "" {
			m["title"] = it.Name
		}
		if it.UpdatedAt != "" {
			m["updatedAt"] = it.UpdatedAt
		}
		items = append(items, m)
	}
	out := map[string]any{"sessions": items}
	if end < len(res.Sessions) {
		data := []byte(`{"Offset":` + strconv.Itoa(end) + `,"Cwd":` + string(mustJSON(p.Cwd)) + `}`)
		out["nextCursor"] = base64.RawURLEncoding.EncodeToString(data)
	}
	return out, nil
}
