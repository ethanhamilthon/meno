package agent

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"meno/internal/llm"
)

type Turn struct {
	// Replay is the complete assistant output; subsequent tool turns share its group.
	Replay      *llm.Msg  `json:"replay,omitempty"`
	ReplayGroup string    `json:"replay_group,omitempty"`
	Role        string    `json:"role"`
	Text        string    `json:"text"`
	Thinking    string    `json:"thinking,omitempty"`
	CallID      string    `json:"call_id,omitempty"`
	Code        string    `json:"code,omitempty"`
	Output      string    `json:"output,omitempty"`
	Err         string    `json:"err,omitempty"`
	Time        time.Time `json:"time"`
	// System is the <system-event> block sent in front of a user message. It is
	// part of the request history but never shown to the user.
	System string `json:"system,omitempty"`
}

type RequestUsage struct {
	Provider string    `json:"provider"`
	Model    string    `json:"model"`
	Usage    llm.Usage `json:"usage"`
	Cost     *float64  `json:"cost,omitempty"`
	Time     time.Time `json:"time"`
}

type Metrics struct {
	LastUsage llm.Usage `json:"last_usage"`
	Usage     llm.Usage `json:"usage"`
	Spent     float64   `json:"spent"`
	Context   int       `json:"context"`
	Unpriced  bool      `json:"unpriced,omitempty"`
}

type Session struct {
	Requests  []RequestUsage `json:"requests,omitempty"`
	Metrics   Metrics        `json:"metrics"`
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Cwd       string         `json:"cwd"`
	Model     string         `json:"model"`
	UpdatedAt time.Time      `json:"updated_at"`
	Summary   string         `json:"summary,omitempty"`
	Log       *SafeLog       `json:"-"`
	Turns     []Turn         `json:"turns"`
}

func (s *Session) ensureLog() *SafeLog {
	if s.Log == nil {
		s.Log = &SafeLog{turns: s.Turns}
	}
	return s.Log
}

func (s *Session) Append(t Turn) { s.ensureLog().Append(t) }

func (s *Session) Len() int         { return s.ensureLog().Len() }
func (s *Session) Snapshot() []Turn { return s.ensureLog().Snapshot() }

func (s *Session) Replace(turns []Turn) { s.ensureLog().Replace(turns) }

// SetSystem sets the system-event block of turn i.
func (s *Session) SetSystem(i int, system string) { s.ensureLog().SetSystem(i, system) }

type SessionInfo struct {
	ID        string
	Title     string
	Cwd       string
	UpdatedAt time.Time
	Turns     int
	JSRuns    int
}

func NewID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

func NewSession(cwd, model string) *Session {
	s := &Session{ID: NewID(), Cwd: cwd, Model: model, UpdatedAt: time.Now()}
	s.ensureLog()
	return s
}

// SaveSession persists the session into dir/sessions.db in one transaction.
func SaveSession(dir string, s *Session) error {
	if s.ID == "" {
		return errors.New("session id is empty")
	}
	s.UpdatedAt = time.Now()
	s.Turns = s.Snapshot()
	if s.Title == "" {
		s.Title = deriveTitle(s)
	}
	db, err := openDB(dir)
	if err != nil {
		return err
	}
	return writeSession(db, s)
}

func LoadSession(dir, id string) (*Session, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}
	db, err := openDB(dir)
	if err != nil {
		return nil, err
	}
	s, err := readSession(db, id)
	if err != nil {
		return nil, err
	}
	s.ensureLog()
	return s, nil
}

// ListSessions returns every stored session, newest first.
func ListSessions(dir string) ([]SessionInfo, error) { return ListRecentSessions(dir, -1) }

// ListRecentSessions returns the n most recently updated sessions (all when n < 0).
func ListRecentSessions(dir string, n int) ([]SessionInfo, error) {
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	db, err := openDB(dir)
	if err != nil {
		return nil, err
	}
	return listSessions(db, n)
}

// DisplayTitle is the title shown in lists: the saved one, or one made from the
// first user message.
func (s *Session) DisplayTitle() string {
	if s.Title != "" {
		return s.Title
	}
	return deriveTitle(&Session{Turns: s.Snapshot()})
}

func deriveTitle(s *Session) string {
	for _, t := range s.Turns {
		if t.Role == llm.RoleUser && t.Text != "" {
			r := []rune(t.Text)
			if len(r) > 60 {
				return string(r[:60])
			}
			return string(r)
		}
	}
	return "untitled"
}

func toolResult(t Turn) llm.Msg {
	res := t.Output
	if t.Err != "" {
		res += "\nERROR: " + t.Err
	}
	return llm.Msg{Role: llm.RoleTool, CallID: t.CallID, Text: res}
}

func (s *Session) ToMsgs() []llm.Msg {
	var out []llm.Msg
	if s.Summary != "" {
		out = append(out, llm.Msg{Role: llm.RoleUser, Text: "Summary of earlier conversation:\n" + s.Summary})
	}
	turns := s.Snapshot()
	for i := 0; i < len(turns); i++ {
		t := turns[i]
		if t.Replay != nil {
			out = append(out, *t.Replay)
			if t.CallID != "" {
				out = append(out, toolResult(t))
			}
			for i+1 < len(turns) && t.ReplayGroup != "" && turns[i+1].ReplayGroup == t.ReplayGroup {
				i++
				if turns[i].CallID != "" {
					out = append(out, toolResult(turns[i]))
				}
			}
			continue
		}
		switch {
		case t.Role == llm.RoleUser:
			out = append(out, llm.Msg{Role: llm.RoleUser, Text: withSystem(t)})
		case t.Code != "" || t.CallID != "":
			out = append(out, llm.Msg{Role: llm.RoleAssistant, Text: t.Text, CallID: t.CallID, Code: t.Code})
			out = append(out, toolResult(t))
		case t.Role == llm.RoleAssistant:
			out = append(out, llm.Msg{Role: llm.RoleAssistant, Text: t.Text})
		}
	}
	return out
}

// withSystem is the text of a user turn as the model sees it: the system-event
// block, when there is one, then the user's words.
func withSystem(t Turn) string {
	if t.System == "" {
		return t.Text
	}
	return t.System + "\n\n" + t.Text
}
