package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func runFileCommand(name string, args []string, streams IO, jsonOut bool) (int, error) {
	switch name {
	case "read":
		return readFile(args, streams, jsonOut)
	case "write":
		return writeFile(args, streams, jsonOut)
	case "edit":
		return editFile(args, streams, jsonOut)
	case "search":
		return searchFiles(args, streams, jsonOut)
	}
	return ExitError, unknown(name, []string{"read", "edit", "write", "search"})
}

type numberedLine struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

func readFile(args []string, streams IO, jsonOut bool) (int, error) {
	_, switches, pos, err := parseFlags(args, nil, map[string]bool{"hash": true})
	if err != nil {
		return 0, err
	}
	if len(pos) < 1 || len(pos) > 3 {
		return 0, usageError("usage: meno read FILE [START END] [--hash]")
	}
	start, end := 1, 0
	if len(pos) >= 2 {
		start, err = intFlag("start", pos[1], 0, 1)
		if err != nil {
			return 0, err
		}
	}
	if len(pos) == 3 {
		end, err = intFlag("end", pos[2], 0, 1)
		if err != nil {
			return 0, err
		}
	}
	var data []byte
	if pos[0] == "-" {
		data, err = io.ReadAll(streams.In)
	} else {
		data, err = os.ReadFile(cleanPath(pos[0], streams))
	}
	if err != nil {
		return 0, err
	}
	lines := splitFileLines(data)
	if end == 0 {
		end = len(lines)
	}
	if end < start {
		return 0, usageError("END must be greater than or equal to START")
	}
	if start > len(lines) && !(len(lines) == 0 && start == 1) {
		return 0, fmt.Errorf("start line %d is past the end of the file (%d lines)", start, len(lines))
	}
	end = min(end, len(lines))
	selected := make([]numberedLine, 0, max(0, end-start+1))
	for i := max(0, start-1); i < end; i++ {
		text := strings.TrimSuffix(strings.TrimSuffix(lines[i], "\n"), "\r")
		selected = append(selected, numberedLine{Number: i + 1, Text: text})
	}
	if jsonOut {
		return 0, emit(streams, true, map[string]any{"file": pos[0], "lines": selected, "sha256": hashBytes(data), "hashRequested": switches["hash"]}, "")
	}
	for _, line := range selected {
		if _, err := fmt.Fprintf(streams.Out, "%d\t%s\n", line.Number, line.Text); err != nil {
			return 0, err
		}
	}
	if switches["hash"] {
		_, err = fmt.Fprintf(streams.Out, "sha256 %s\n", hashBytes(data))
	}
	return 0, err
}
