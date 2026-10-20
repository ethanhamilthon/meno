package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type replacement struct {
	Old string `json:"oldText"`
	New string `json:"newText"`
}

func editFile(args []string, streams IO, jsonOut bool) (int, error) {
	if len(args) == 0 {
		return 0, usageError("usage: meno edit FILE OLD NEW")
	}
	if args[0] == "-" && len(args) >= 2 && args[1] == "--json" {
		args = args[1:]
	}
	if len(args) >= 2 && args[0] != "-" && args[1] == "--json" {
		if len(args) < 3 {
			return 0, usageError("usage: meno edit FILE --json [--expect-hash HASH]")
		}
		flags, _, pos, err := parseFlags(args[2:], map[string]bool{"expect-hash": true}, nil)
		if err != nil {
			return 0, err
		}
		if len(pos) != 0 {
			return 0, usageError("usage: meno edit FILE --json [--expect-hash HASH]")
		}
		return editJSON(args[0], flags["expect-hash"], streams, jsonOut)
	}
	if len(args) != 3 {
		return 0, usageError("usage: meno edit FILE OLD NEW")
	}
	if args[1] == "" {
		return 0, usageError("OLD must not be empty")
	}
	path := cleanPath(args[0], streams)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if strings.Count(string(data), args[1]) != 1 {
		return 0, fmt.Errorf("old text %s", matchIssue(string(data), args[1]))
	}
	next := strings.Replace(string(data), args[1], args[2], 1)
	if err := atomicEdit(path, data, []byte(next)); err != nil {
		return 0, err
	}
	return editResult(args[0], []byte(next), 1, streams, jsonOut)
}

func editJSON(file, expected string, streams IO, jsonOut bool) (int, error) {
	input, err := io.ReadAll(streams.In)
	if err != nil {
		return 0, err
	}
	var edits []replacement
	if err := json.Unmarshal(input, &edits); err != nil {
		return 0, usageError("invalid edit JSON: " + err.Error())
	}
	path := cleanPath(file, streams)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if expected == "" {
		return 0, usageError("--expect-hash is required for JSON edits")
	}
	if got := hashBytes(data); got != expected {
		return 0, fmt.Errorf("%s changed since it was inspected (hash is now %s); read it again", file, got)
	}
	next, err := applyEdits(string(data), edits)
	if err != nil {
		return 0, err
	}
	if err := atomicEdit(path, data, []byte(next)); err != nil {
		return 0, err
	}
	return editResult(file, []byte(next), len(edits), streams, jsonOut)
}

func editResult(file string, data []byte, count int, streams IO, jsonOut bool) (int, error) {
	result := map[string]any{"file": file, "sha256": hashBytes(data), "replacements": count}
	if jsonOut {
		return 0, emit(streams, true, result, "")
	}
	_, err := fmt.Fprintf(streams.Out, "sha256 %s %d replacements\n", result["sha256"], count)
	return 0, err
}
