package agent

import (
	"errors"
	"strings"
	"unicode"
)

// MaxTitleWords is the longest session name; agents are told to aim for four.
const MaxTitleWords = 6

// NormalizeTitle validates a session name given by the agent: trimmed, one
// line, no control characters, one to six words.
func NormalizeTitle(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("session name must not be empty")
	}
	for _, r := range name {
		if r == '\n' || r == '\r' {
			return "", errors.New("session name must be a single line")
		}
		if unicode.IsControl(r) {
			return "", errors.New("session name must not contain control characters")
		}
	}
	if n := len(strings.Fields(name)); n > MaxTitleWords {
		return "", errors.New("session name is too long: use at most 6 words, ideally 4")
	}
	return name, nil
}

// SessionKind returns the kind and parent of a stored session.
func SessionKind(dir, id string) (kind, parent string, err error) {
	d, err := openDB(dir)
	if err != nil {
		return "", "", err
	}
	err = d.QueryRow("SELECT kind, parent_id FROM sessions WHERE id = ?", id).Scan(&kind, &parent)
	if err != nil {
		return "", "", errors.New("session " + id + " not found")
	}
	return kind, parent, nil
}

// RenameSession stores a new title for a session at once. The session row is
// created if it was not saved yet.
func RenameSession(dir, id, title string) error {
	d, err := openDB(dir)
	if err != nil {
		return err
	}
	_, err = d.Exec(`INSERT INTO sessions(id,title,updated_at) VALUES(?,?,?)
		ON CONFLICT(id) DO UPDATE SET title = excluded.title`, id, title, ts(timeNow()))
	return err
}
