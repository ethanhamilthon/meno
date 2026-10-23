package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// fail prints one CLI error line and returns its documented exit code.
func fail(streams IO, err error) int {
	code := ExitError
	if err == nil {
		return ExitOK
	}
	if isAborted(err) {
		code = ExitAborted
	}
	var usage usageFailure
	if errors.As(err, &usage) {
		code = ExitUsage
	}
	var e *APIError
	if errors.As(err, &e) {
		if e.Code == "forbidden" {
			code = ExitForbidden
		}
		if e.Code == "wait_timeout" || e.Code == "timeout" {
			code = ExitWaitTimeout
		}
	}
	fmt.Fprintf(streams.Err, "meno: %v\n", err)
	return code
}

// usageFailure marks a parse or argument validation error.
type usageFailure struct{ message string }

func (e usageFailure) Error() string  { return e.message }
func usageError(message string) error { return usageFailure{message} }

// printHelp writes the selected embedded help page.
func printHelp(streams IO, topic string) int {
	text, err := Text(topic)
	if err != nil {
		return fail(streams, err)
	}
	_, _ = io.WriteString(streams.Out, text)
	return ExitOK
}

// emit writes JSON or the matching plain-text result.
func emit(streams IO, jsonOut bool, value any, plain string) error {
	if jsonOut {
		enc := json.NewEncoder(streams.Out)
		enc.SetEscapeHTML(false)
		return enc.Encode(value)
	}
	_, err := io.WriteString(streams.Out, plain)
	return err
}

// apiRequired reports when a daemon adapter is not available.
func apiRequired(api API) error {
	if api == nil {
		return fmt.Errorf("daemon API is not configured")
	}
	return nil
}

// cleanPath resolves a file argument relative to the CLI working directory.
func cleanPath(p string, streams IO) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(streams.Dir, p)
	}
	return filepath.Clean(p)
}
