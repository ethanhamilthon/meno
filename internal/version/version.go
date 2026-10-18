// Package version holds the single source of truth for the meno version.
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var raw string

// Number returns the version without a "v" prefix, for example "0.1.1".
func Number() string { return strings.TrimSpace(raw) }

// Label returns the version for display, for example "v0.1.1".
func Label() string { return "v" + Number() }
