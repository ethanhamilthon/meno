package agent

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"meno/internal/config"
	"meno/internal/llm"
)

type Turn struct {
	Role     string    `json:"role"`
	Text     string    `json:"text"`
	Thinking string    `json:"thinking,omitempty"`
	CallID   string    `json:"call_id,omitempty"`
	Code     string    `json:"code,omitempty"`
	Output   string    `json:"output,omitempty"`
	Err      string    `json:"err,omitempty"`
	Time     time.Time `json:"time"`
}

type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Cwd       string    `json:"cwd"`
	Model     string    `json:"model"`
	Turns     []Turn    `json:"turns"`
	UpdatedAt time.Time `json:"updated_at"`
	Summary   string    `json:"summary,omitempty"`
}

type SessionInfo struct {
	ID        string
	Title     string
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
	return &Session{ID: NewID(), Cwd: cwd, Model: model, UpdatedAt: time.Now()}
}

func SaveSession(dir string, s *Session) error {
	if s.ID == "" {
		return errors.New("session id is empty")
	}
	s.UpdatedAt = time.Now()
	if s.Title == "" {
		s.Title = deriveTitle(s)
	}
	return config.SaveJSON(filepath.Join(dir, s.ID+".json"), s)
}

func LoadSession(dir, id string) (*Session, error) {
	var s Session
	if err := config.LoadJSON(filepath.Join(dir, id+".json"), &s); err != nil {
		return nil, err
	}
	if s.ID == "" {
		return nil, os.ErrNotExist
	}
	return &s, nil
}

func ListSessions(dir string) ([]SessionInfo, error) {
	ents, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []SessionInfo
	for _, e := range ents {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		s, err := LoadSession(dir, e.Name()[:len(e.Name())-5])
		if err != nil {
			continue
		}
		js := 0
		for _, t := range s.Turns {
			if t.Code != "" {
				js++
			}
		}
		out = append(out, SessionInfo{ID: s.ID, Title: s.Title, UpdatedAt: s.UpdatedAt, Turns: len(s.Turns), JSRuns: js})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
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

func (s *Session) ToMsgs() []llm.Msg {
	var out []llm.Msg
	if s.Summary != "" {
		out = append(out, llm.Msg{Role: llm.RoleUser, Text: "Summary of earlier conversation:\n" + s.Summary})
	}
	for _, t := range s.Turns {
		switch {
		case t.Role == llm.RoleUser:
			out = append(out, llm.Msg{Role: llm.RoleUser, Text: t.Text})
		case t.Code != "":
			out = append(out, llm.Msg{Role: llm.RoleAssistant, Text: t.Text, CallID: t.CallID, Code: t.Code})
			res := t.Output
			if t.Err != "" {
				res += "\nERROR: " + t.Err
			}
			out = append(out, llm.Msg{Role: llm.RoleTool, CallID: t.CallID, Text: res})
		case t.Role == llm.RoleAssistant:
			out = append(out, llm.Msg{Role: llm.RoleAssistant, Text: t.Text})
		}
	}
	return out
}
