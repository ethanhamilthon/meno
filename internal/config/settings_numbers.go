package config

// MaxLines bounds the "lines <n>" limits.
const (
	MaxLines = 50
	// The auto-compact threshold, in percent of the context window.
	MinCompactPercent = 10
	MaxCompactPercent = 95
	compactStep       = 5
)

// NumOption is the choice of a setting that carries a number.
func NumOption(key string) string {
	switch key {
	case "bash_code", "bash_output", "thinking":
		return "lines"
	case "compact_percent":
		return "percent"
	}
	return ""
}

// NumFor returns the number that belongs to a setting's numeric choice.
func (s *Settings) NumFor(key string) int {
	switch key {
	case "bash_code":
		return s.BashLines
	case "bash_output":
		return s.BashOutputLines
	case "thinking":
		return s.ThinkLines
	case "compact_percent":
		return s.CompactPercent
	default:
		return 0
	}
}

// AdjustNum changes the number of a setting's numeric choice by one step in the
// direction of delta and returns the new value.
func (s *Settings) AdjustNum(key string, delta int) int {
	if NumOption(key) == "" {
		return 0
	}
	switch key {
	case "bash_code":
		s.BashLines = min(MaxLines, max(1, s.BashLines+delta))
		return s.BashLines
	case "bash_output":
		s.BashOutputLines = min(MaxLines, max(1, s.BashOutputLines+delta))
		return s.BashOutputLines
	case "compact_percent":
		s.CompactPercent = min(MaxCompactPercent, max(MinCompactPercent, s.CompactPercent+delta*compactStep))
		return s.CompactPercent
	case "thinking":
		s.ThinkLines = min(MaxLines, max(1, s.ThinkLines+delta))
		return s.ThinkLines
	default:
		return 0
	}
}
