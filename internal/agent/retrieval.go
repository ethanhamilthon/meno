package agent

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// Retrieval limits.
const (
	DefaultPage   = 20
	MaxPage       = 100
	DefaultRead   = 50
	ReadByteLimit = 64 << 10
	excerptRunes  = 200
)

// Query selects sessions. All local main sessions are searched; Workdir is only
// a filter on the exact normalized directory, not a security boundary.
type Query struct {
	Workdir       string
	After, Before time.Time // inclusive; zero = unbounded
	Limit         int
	Cursor        string
}

// SessionRef is a session as retrieval shows it. No credentials, no bytes.
type SessionRef struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Workdir         string `json:"workdir"`
	LastMessageTime string `json:"lastMessageTime"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	MessageID       int64  `json:"messageId,omitempty"`
	Excerpt         string `json:"excerpt,omitempty"`
}

// Page is one page of results with the cursor of the next, "" at the end.
type Page struct {
	Items      []SessionRef `json:"items"`
	NextCursor string       `json:"nextCursor"`
}

func (q Query) limit() int {
	switch {
	case q.Limit <= 0:
		return DefaultPage
	case q.Limit > MaxPage:
		return MaxPage
	}
	return q.Limit
}

// A cursor is "<time>|<id>|<hid>" of the last item returned, base64-encoded.
func encodeCursor(when, id string, hid int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s|%s|%d", when, id, hid)))
}

func decodeCursor(c string) (when, id string, hid int64, err error) {
	if c == "" {
		return "", "", 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(c)
	parts := strings.Split(string(raw), "|")
	if err != nil || len(parts) != 3 {
		return "", "", 0, fmt.Errorf("invalid cursor")
	}
	fmt.Sscan(parts[2], &hid)
	return parts[0], parts[1], hid, nil
}

// ListSessionsPage lists main sessions, newest message first, stable by ID.
func ListSessionsPage(dir string, q Query) (Page, error) {
	d, err := openDB(dir)
	if err != nil {
		return Page{}, err
	}
	when, id, _, err := decodeCursor(q.Cursor)
	if err != nil {
		return Page{}, err
	}
	args := []any{}
	where := "kind = 'main'"
	if q.Workdir != "" {
		where += " AND cwd = ?"
		args = append(args, q.Workdir)
	}
	if !q.After.IsZero() {
		where += " AND updated_at >= ?"
		args = append(args, ts(q.After))
	}
	if !q.Before.IsZero() {
		where += " AND updated_at <= ?"
		args = append(args, ts(q.Before))
	}
	if when != "" {
		where += " AND (updated_at < ? OR (updated_at = ? AND id > ?))"
		args = append(args, when, when, id)
	}
	args = append(args, q.limit()+1)
	rows, err := d.Query("SELECT id,title,cwd,updated_at,provider,model FROM sessions WHERE "+where+" ORDER BY updated_at DESC, id ASC LIMIT ?", args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	p := Page{Items: []SessionRef{}}
	for rows.Next() {
		var r SessionRef
		if err := rows.Scan(&r.ID, &r.Title, &r.Workdir, &r.LastMessageTime, &r.Provider, &r.Model); err != nil {
			return Page{}, err
		}
		if len(p.Items) == q.limit() {
			last := p.Items[len(p.Items)-1]
			p.NextCursor = encodeCursor(last.LastMessageTime, last.ID, 0)
			break
		}
		p.Items = append(p.Items, r)
	}
	return p, rows.Err()
}

// escapeLike makes s literal inside a LIKE pattern with ESCAPE '\'.
func escapeLike(s string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(s)
}

func excerpt(text, query string) string {
	i := strings.Index(strings.ToLower(text), strings.ToLower(query))
	if i < 0 {
		i = 0
	}
	runes := []rune(text[i:])
	pre := []rune(text[:i])
	if len(pre) > 40 {
		pre = pre[len(pre)-40:]
	}
	s := string(pre) + string(runes[:min(len(runes), excerptRunes)])
	if len(runes) > excerptRunes {
		s += "…"
	}
	return s
}

// SearchSessions finds messages whose text contains query (case-insensitive,
// literal), or sessions whose title does. Results are ordered by time, newest
// first, then session and message ID. Child sessions are never searched.
func SearchSessions(dir, query string, q Query) (Page, error) {
	if strings.TrimSpace(query) == "" {
		return Page{}, fmt.Errorf("empty query: use sessions.list to browse sessions")
	}
	d, err := openDB(dir)
	if err != nil {
		return Page{}, err
	}
	when, id, hid, err := decodeCursor(q.Cursor)
	if err != nil {
		return Page{}, err
	}
	pat := "%" + escapeLike(query) + "%"
	args := []any{pat}
	where := "s.kind = 'main'"
	if q.Workdir != "" {
		where += " AND s.cwd = ?"
		args = append(args, q.Workdir)
	}
	if !q.After.IsZero() {
		where += " AND m.time >= ?"
		args = append(args, ts(q.After))
	}
	if !q.Before.IsZero() {
		where += " AND m.time <= ?"
		args = append(args, ts(q.Before))
	}
	if when != "" {
		where += " AND (m.time < ? OR (m.time = ? AND (m.sid > ? OR (m.sid = ? AND m.hid > ?))))"
		args = append(args, when, when, id, id, hid)
	}
	args = append(args, q.limit()+1)
	rows, err := d.Query(`SELECT s.id,s.title,s.cwd,s.provider,s.model,m.hid,m.time,m.text FROM (
		SELECT session_id AS sid, hid, time, text FROM turns
		UNION ALL
		SELECT id, 0, updated_at, title FROM sessions
		UNION ALL
		SELECT session_id, hid, json_extract(turn,'$.time'), json_extract(turn,'$.text') FROM history
	) m JOIN sessions s ON s.id = m.sid
	WHERE m.text LIKE ? ESCAPE '\' AND `+where+`
	ORDER BY m.time DESC, m.sid ASC, m.hid ASC LIMIT ?`, args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	p := Page{Items: []SessionRef{}}
	for rows.Next() {
		var r SessionRef
		var text string
		if err := rows.Scan(&r.ID, &r.Title, &r.Workdir, &r.Provider, &r.Model, &r.MessageID, &r.LastMessageTime, &text); err != nil {
			return Page{}, err
		}
		if len(p.Items) == q.limit() {
			last := p.Items[len(p.Items)-1]
			p.NextCursor = encodeCursor(last.LastMessageTime, last.ID, last.MessageID)
			break
		}
		r.Excerpt = excerpt(text, query)
		p.Items = append(p.Items, r)
	}
	return p, rows.Err()
}
