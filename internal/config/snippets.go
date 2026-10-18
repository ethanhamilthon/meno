package config

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Snippet is a named piece of text saved from the terminal and pasted into the
// composer later. Snippets are global: every session sees the same list.
type Snippet struct {
	Name      string `json:"name"`
	Text      string `json:"text"`
	UpdatedAt string `json:"updatedAt"`
}

const (
	maxSnippetName = 40
	maxSnippetText = 64 << 10
)

var snippetNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// CheckSnippetName reports whether name can be used as a snippet key.
func CheckSnippetName(name string) error {
	switch {
	case name == "":
		return errors.New("snippet name is empty")
	case len(name) > maxSnippetName:
		return fmt.Errorf("snippet name is longer than %d characters", maxSnippetName)
	case !snippetNameRe.MatchString(name):
		return errors.New("snippet name: use letters, digits, dot, dash or underscore")
	}
	return nil
}

// LoadSnippets lists every snippet, sorted by name.
func (s *Store) LoadSnippets() ([]Snippet, error) {
	d, err := s.DB()
	if err != nil {
		return nil, err
	}
	rows, err := d.Query("SELECT name, text, updated_at FROM snippets ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Snippet
	for rows.Next() {
		var sn Snippet
		if err := rows.Scan(&sn.Name, &sn.Text, &sn.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// Snippet returns one snippet by name.
func (s *Store) Snippet(name string) (Snippet, error) {
	d, err := s.DB()
	if err != nil {
		return Snippet{}, err
	}
	var sn Snippet
	err = d.QueryRow("SELECT name, text, updated_at FROM snippets WHERE name = ?", name).
		Scan(&sn.Name, &sn.Text, &sn.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Snippet{}, fmt.Errorf("no snippet named %q", name)
	}
	return sn, err
}

// SaveSnippet creates a snippet or replaces the text of an existing one.
func (s *Store) SaveSnippet(name, text string) error {
	if err := CheckSnippetName(name); err != nil {
		return err
	}
	if len(text) > maxSnippetText {
		return fmt.Errorf("snippet is longer than %d bytes", maxSnippetText)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return s.tx(func(t *sql.Tx) error {
		_, err := t.Exec(`INSERT INTO snippets(name,text,created_at,updated_at) VALUES(?,?,?,?)
			ON CONFLICT(name) DO UPDATE SET text=excluded.text, updated_at=excluded.updated_at`,
			name, text, now, now)
		return err
	})
}

// RenameSnippet renames a snippet, keeping its text.
func (s *Store) RenameSnippet(old, name string) error {
	if err := CheckSnippetName(name); err != nil {
		return err
	}
	if old == name {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return s.tx(func(t *sql.Tx) error {
		res, err := t.Exec("UPDATE snippets SET name = ?, updated_at = ? WHERE name = ?", name, now, old)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				return fmt.Errorf("a snippet named %q already exists", name)
			}
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("no snippet named %q", old)
		}
		return nil
	})
}

// DeleteSnippet removes a snippet. Deleting a missing name is not an error.
func (s *Store) DeleteSnippet(name string) error {
	return s.tx(func(t *sql.Tx) error {
		_, err := t.Exec("DELETE FROM snippets WHERE name = ?", name)
		return err
	})
}
