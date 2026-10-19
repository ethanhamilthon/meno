package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func writeFile(args []string, streams IO, jsonOut bool) (int, error) {
	_, switches, pos, err := parseFlags(args, nil, map[string]bool{"overwrite": true})
	if err != nil {
		return 0, err
	}
	if len(pos) != 1 {
		return 0, usageError("usage: meno write FILE [--overwrite]")
	}
	data, err := io.ReadAll(streams.In)
	if err != nil {
		return 0, err
	}
	path := cleanPath(pos[0], streams)
	defer lockFile(path)()
	if !switches["overwrite"] {
		if _, statErr := os.Stat(path); statErr == nil {
			old, readErr := os.ReadFile(path)
			if readErr != nil {
				return 0, readErr
			}
			return 0, fmt.Errorf("%s already exists (sha256 %s, %d lines); use --overwrite to replace it", pos[0], hashBytes(old), len(splitFileLines(old)))
		} else if !os.IsNotExist(statErr) {
			return 0, statErr
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return 0, err
	}
	result := map[string]any{"file": pos[0], "sha256": hashBytes(data), "lines": len(splitFileLines(data))}
	if jsonOut {
		return 0, emit(streams, true, result, "")
	}
	_, err = fmt.Fprintf(streams.Out, "sha256 %s %d lines\n", result["sha256"], result["lines"])
	return 0, err
}
