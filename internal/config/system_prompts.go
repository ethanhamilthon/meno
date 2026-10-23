package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxSystemPromptBytes bounds one custom system prompt file.
const MaxSystemPromptBytes = 64 << 10

// DefaultSystemPrompt is the name the settings show for "no custom prompt".
const DefaultSystemPrompt = "default"

// SystemPromptsDir holds the custom system prompts: one NAME.md per prompt.
func (p Paths) SystemPromptsDir() string { return filepath.Join(p.Home, "system-prompts") }

// ValidSystemPromptName reports whether name can name a prompt file: 1 to 64
// letters, digits, dots, dashes or underscores, not starting with a dot. It
// never contains a path separator, so a name cannot leave the directory.
func ValidSystemPromptName(name string) error {
	if name == "" || len(name) > 64 || strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid system prompt name %q: use 1 to 64 letters, digits, dots, dashes or underscores", name)
	}
	for _, r := range name {
		ok := r == '.' || r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !ok {
			return fmt.Errorf("invalid system prompt name %q: use 1 to 64 letters, digits, dots, dashes or underscores", name)
		}
	}
	return nil
}

// ErrSystemPromptMissing is returned when no file belongs to a prompt name.
var ErrSystemPromptMissing = errors.New("system prompt not found")

// LoadSystemPrompt reads the custom system prompt called name. The text is
// trimmed; an empty or oversized file is an error.
func (p Paths) LoadSystemPrompt(name string) (string, error) {
	if err := ValidSystemPromptName(name); err != nil {
		return "", err
	}
	file := filepath.Join(p.SystemPromptsDir(), name+".md")
	b, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%w: %s (expected %s)", ErrSystemPromptMissing, name, file)
	}
	if err != nil {
		return "", err
	}
	if len(b) > MaxSystemPromptBytes {
		return "", fmt.Errorf("system prompt %s is larger than %d bytes", name, MaxSystemPromptBytes)
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		return "", fmt.Errorf("system prompt %s is empty", name)
	}
	return text, nil
}

// ListSystemPrompts returns the names of the usable prompt files, sorted.
func (p Paths) ListSystemPrompts() []string {
	entries, err := os.ReadDir(p.SystemPromptsDir())
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".md")
		if !ok || e.IsDir() || ValidSystemPromptName(name) != nil {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
