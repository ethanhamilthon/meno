package agent

import (
	"errors"
	"strings"
	"unicode"
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
