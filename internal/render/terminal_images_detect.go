package render

import (
	"os"
	"strings"
)

// Detect picks a protocol for the terminal from its environment. It is a hint,
// not a confirmation: meno cannot ask the terminal (replies would reach the
// input as typed characters), so a terminal that is not recognized gets text
// only, and the images.protocol setting forces a protocol. Inside tmux or
// screen graphics are off unless forced, because passthrough needs
// configuration meno cannot verify.
func Detect(env func(string) string) string {
	if env == nil {
		env = os.Getenv
	}
	if env("TMUX") != "" || strings.HasPrefix(env("TERM"), "screen") || strings.HasPrefix(env("TERM"), "tmux") {
		return Off
	}
	term, prog := env("TERM"), env("TERM_PROGRAM")
	switch {
	case env("KITTY_WINDOW_ID") != "" || term == "xterm-kitty" || term == "xterm-ghostty" || prog == "ghostty" || prog == "WezTerm":
		return Kitty
	case prog == "iTerm.app" || env("LC_TERMINAL") == "iTerm2":
		return ITerm
	case strings.Contains(term, "sixel") || term == "foot" || term == "mlterm" || prog == "contour":
		return Sixel
	}
	return Off
}

// Resolve applies the setting: auto uses Detect, anything else is taken as is.
func Resolve(setting string, env func(string) string) string {
	if setting == "" || setting == Auto {
		return Detect(env)
	}
	if Valid(setting) {
		return setting
	}
	return Off
}
