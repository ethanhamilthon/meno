package config

const (
	// The auto-compact threshold, in percent of the context window.
	MinCompactPercent = 10
	MaxCompactPercent = 95
	compactStep       = 5
)

// NumOption is the choice of a setting that carries a number.
func NumOption(key string) string {
	switch key {
	case "compact_percent":
		return "percent"
	}
	return ""
}

// NumFor returns the number that belongs to a setting's numeric choice.
func (s *Settings) NumFor(key string) int {
	switch key {
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
	case "compact_percent":
		s.CompactPercent = min(MaxCompactPercent, max(MinCompactPercent, s.CompactPercent+delta*compactStep))
		return s.CompactPercent
	default:
		return 0
	}
}
