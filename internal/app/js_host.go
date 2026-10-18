package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"meno/internal/agent"
	"path/filepath"
	"time"
)

// toAny turns any JSON-encodable value into plain maps and slices, so the VM
// shows the json field names instead of Go struct fields.
func toAny(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	err = json.Unmarshal(b, &out)
	return out, err
}

// sessionHost implements jsvm.SessionHost for one session. It runs on the
// agent's goroutine, so anything that touches UI state goes through onUI.
type sessionHost struct {
	a     *App
	sid   string
	sess  *agent.Session // set for a subagent, whose session is not in a.rts
	child bool
}

func (h sessionHost) Rename(name string) (string, error) {
	if h.child {
		return "", errors.New("session.rename is not available to a subagent")
	}
	title, err := agent.NormalizeTitle(name)
	if err != nil {
		return "", err
	}
	if err := agent.RenameSession(h.a.paths.Home, h.sid, title); err != nil {
		return "", err
	}
	h.a.onUI(func() {
		if rt := h.a.rts[h.sid]; rt != nil {
			rt.Session.Title = title
		}
	})
	return title, nil
}

func (h sessionHost) Compact(summary string, keep int) error {
	sess := h.sess
	if sess != nil {
		return sess.ScheduleCompaction(summary, keep)
	}
	h.a.onUI(func() {
		if rt := h.a.rts[h.sid]; rt != nil {
			sess = rt.Session
		}
	})
	if sess == nil {
		return errors.New("session is gone")
	}
	return sess.ScheduleCompaction(summary, keep)
}

func queryFrom(opts map[string]any) (agent.Query, error) {
	q := agent.Query{}
	for k, v := range opts {
		switch k {
		case "workdir":
			s, ok := v.(string)
			if !ok {
				return q, errors.New("workdir must be a string")
			}
			if s != "" {
				q.Workdir = filepath.Clean(expandHome(s))
			}
		case "after", "before":
			s, ok := v.(string)
			if !ok {
				return q, fmt.Errorf("%s must be an ISO timestamp string", k)
			}
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return q, fmt.Errorf("%s must be an ISO timestamp: %v", k, err)
			}
			if k == "after" {
				q.After = t.UTC()
			} else {
				q.Before = t.UTC()
			}
		case "limit":
			n, ok := v.(int64)
			if !ok || n < 1 {
				return q, errors.New("limit must be a positive integer")
			}
			q.Limit = int(n)
		case "cursor":
			s, _ := v.(string)
			q.Cursor = s
		default:
			return q, fmt.Errorf("unknown option %q", k)
		}
	}
	return q, nil
}

var errChildSessions = errors.New("a subagent can read only its own conversation")

func (h sessionHost) List(opts map[string]any) (any, error) {
	if h.child {
		return nil, errChildSessions
	}
	q, err := queryFrom(opts)
	if err != nil {
		return nil, err
	}
	p, err := agent.ListSessionsPage(h.a.paths.Home, q)
	if err != nil {
		return nil, err
	}
	return toAny(p)
}

func (h sessionHost) Search(query string, opts map[string]any) (any, error) {
	if h.child {
		return nil, errChildSessions
	}
	q, err := queryFrom(opts)
	if err != nil {
		return nil, err
	}
	p, err := agent.SearchSessions(h.a.paths.Home, query, q)
	if err != nil {
		return nil, err
	}
	return toAny(p)
}

func (h sessionHost) Read(id string, opts map[string]any) (any, error) {
	o := agent.ReadOptions{}
	for k, v := range opts {
		switch k {
		case "from":
			n, ok := v.(int64)
			if !ok || n < 0 {
				return nil, errors.New("from must be a non-negative integer message id")
			}
			o.From = n
		case "limit":
			n, ok := v.(int64)
			if !ok || n < 1 {
				return nil, errors.New("limit must be a positive integer")
			}
			o.Limit = int(n)
		case "includeToolOutput":
			o.IncludeToolOutput, _ = v.(bool)
		case "includeThinking":
			o.IncludeThinking, _ = v.(bool)
		default:
			return nil, fmt.Errorf("unknown option %q", k)
		}
	}
	// Children are internal: only their parent may read one.
	if err := h.checkAccess(id); err != nil {
		return nil, err
	}
	r, err := agent.ReadSessionHistory(h.a.paths.Home, id, o)
	if err != nil {
		return nil, err
	}
	return toAny(r)
}

// checkAccess lets a main session read any main session, and its own children.
func (h sessionHost) checkAccess(id string) error {
	if h.child {
		if id != h.sid {
			return errChildSessions
		}
		return nil
	}
	kind, parent, err := agent.SessionKind(h.a.paths.Home, id)
	if err != nil {
		return err
	}
	if kind == agent.KindChild && parent != h.sid {
		return fmt.Errorf("session %q not found", id)
	}
	return nil
}
