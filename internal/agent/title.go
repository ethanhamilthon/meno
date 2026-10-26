package agent

import (
	"errors"
	"strings"
	"unicode"

	"meno/internal/content"
	"meno/internal/llm"
)

// MaxTitleRunes is the longest session name.
const MaxTitleRunes = 80

// NormalizeTitle validates a session name typed by the user: trimmed, one
// line, no control characters, at most MaxTitleRunes characters.
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
	if len([]rune(name)) > MaxTitleRunes {
		return "", errors.New("session name is too long: use at most 80 characters")
	}
	return name, nil
}

// deriveTitle normalizes object labels before limiting the automatic title.
func deriveTitle(s *Session) string {
	for _, t := range s.Turns {
		if t.Role != llm.RoleUser || t.Kind != "" {
			continue
		}
		if title, ok := titleFromText(t.Text); ok {
			return title
		}
	}
	return "untitled"
}

// TitleFromText derives a short name from the original user input.
func TitleFromText(input string) string { title, _ := titleFromText(input); return title }

func titleFromText(input string) (string, bool) {
	var b strings.Builder
	meaningful := false
	for _, part := range content.Split(input, nil) {
		if part.Obj != nil {
			b.WriteString(part.Obj.Label())
			meaningful = meaningful || part.Obj.Kind != content.Skill
		} else {
			b.WriteString(part.Text)
			meaningful = meaningful || strings.TrimSpace(part.Text) != ""
		}
	}
	text := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) || r == '\u2063' {
			return -1
		}
		return r
	}, b.String())
	text = strings.Join(strings.Fields(text), " ")
	if text == "" || !meaningful {
		return "untitled", false
	}
	r := []rune(text)
	if len(r) > 60 {
		r = r[:60]
	}
	return string(r), true
}
