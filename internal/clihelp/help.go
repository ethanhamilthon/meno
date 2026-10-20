// Package clihelp provides shell-focused help text for meno commands.
package clihelp

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed help/*.md
var content embed.FS

// Text returns embedded help for a group or command. An empty topic means meno.
func Text(topic string) (string, error) {
	if topic == "" {
		topic = "meno"
	}
	topic = strings.TrimPrefix(topic, "meno ")
	if topic == "meno --help" || topic == "--help" {
		topic = "meno"
	}
	topic = strings.ReplaceAll(strings.TrimPrefix(topic, "meno "), " ", "-")
	topic = strings.TrimSuffix(topic, "-help")
	b, err := fs.ReadFile(content, "help/"+topic+".md")
	if err != nil {
		group, _, hasCommand := strings.Cut(topic, "-")
		if hasCommand {
			b, err = fs.ReadFile(content, "help/"+group+".md")
		}
	}
	if err != nil {
		return "", fmt.Errorf("unknown help topic %q", topic)
	}
	return string(b), nil
}
