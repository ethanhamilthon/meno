package process

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RestrictedCommandError is the exact error for unsafe or multiple commands.
const RestrictedCommandError = "restricted mode: one 'meno <group> …' command per call"

// RestrictedCommand is a parsed argv vector whose executable is always meno.
type RestrictedCommand struct {
	Path string
	Args []string
}

// ParseRestrictedCommand validates one command and its allowed group. It returns
// RestrictedCommandError for malformed lines and an allowed-group list for a
// group denial. executable must be the daemon-owned absolute binary path.
func ParseRestrictedCommand(command, executable string, allowedGroups []string) (RestrictedCommand, error) {
	words, err := parseWords(command)
	if err != nil || len(words) < 2 || words[0] != "meno" {
		return RestrictedCommand{}, fmt.Errorf("%s", RestrictedCommandError)
	}
	allowed := false
	for _, group := range allowedGroups {
		if words[1] == group {
			allowed = true
			break
		}
	}
	if !allowed {
		return RestrictedCommand{}, fmt.Errorf("restricted mode: group %q is not allowed (allowed: %s)", words[1], strings.Join(allowedGroups, ", "))
	}
	if !filepath.IsAbs(executable) {
		return RestrictedCommand{}, fmt.Errorf("restricted mode: meno executable must be an absolute path")
	}
	return RestrictedCommand{Path: executable, Args: append([]string(nil), words[1:]...)}, nil
}

func parseWords(input string) ([]string, error) {
	if input == "" {
		return nil, fmt.Errorf("empty command")
	}
	var words []string
	var word strings.Builder
	started := false
	quote := rune(0)
	for _, char := range input {
		if quote == 0 {
			if char == 10 || char == 13 || isForbidden(char) {
				return nil, fmt.Errorf("forbidden syntax")
			}
			if char == ' ' {
				if started {
					words = append(words, word.String())
					word.Reset()
					started = false
				}
				continue
			}
			if char == 39 || char == 34 {
				quote = char
				started = true
				continue
			}
			if !isBare(char) {
				return nil, fmt.Errorf("invalid bare character")
			}
			word.WriteRune(char)
			started = true
			continue
		}
		if char == quote {
			quote = 0
			started = true
			continue
		}
		if char == 13 {
			return nil, fmt.Errorf("invalid carriage return")
		}
		if char == 0 {
			return nil, fmt.Errorf("NUL is not valid in a command")
		}
		if quote == 34 && (char == '$' || char == 96 || char == 92) {
			return nil, fmt.Errorf("forbidden quoted character")
		}
		word.WriteRune(char)
		started = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	if started {
		words = append(words, word.String())
	}
	return words, nil
}

func isForbidden(char rune) bool {
	switch char {
	case ';', '&', '|', '<', '>', '(', ')', '{', '}', '*', '?', '[', ']', '~', '#', '$', 96, 92:
		return true
	default:
		return false
	}
}

func isBare(char rune) bool {
	return char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || strings.ContainsRune("_./:=@%+,-", char)
}
