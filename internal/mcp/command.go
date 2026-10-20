package mcp

import (
	"errors"
	"strings"
)

// parseCommand supports shell-style quoting and escaping, without invoking a
// shell or expanding variables, substitutions, globs, or operators.
func parseCommand(command string) ([]string, error) {
	var args []string
	var word strings.Builder
	var quote byte
	started := false
	for i := 0; i < len(command); i++ {
		c := command[i]
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		if c == '\\' {
			if i+1 == len(command) {
				return nil, errors.New("mcp: trailing command escape")
			}
			next := command[i+1]
			if quote == '"' && next != '"' && next != '\\' && next != '$' && next != '`' && next != '\n' {
				word.WriteByte(c)
				continue
			}
			i++
			if next != '\n' {
				word.WriteByte(next)
				started = true
			}
			continue
		}
		if quote == '"' {
			if c == '"' {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			started = true
		case ' ', '\t', '\n', '\r':
			if started {
				args = append(args, word.String())
				word.Reset()
				started = false
			}
		default:
			word.WriteByte(c)
			started = true
		}
	}
	if quote != 0 {
		return nil, errors.New("mcp: unterminated command quote")
	}
	if started {
		args = append(args, word.String())
	}
	if len(args) == 0 || args[0] == "" {
		return nil, errors.New("mcp: empty command")
	}
	return args, nil
}
