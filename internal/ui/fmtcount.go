package ui

import "fmt"

// CompactCount shortens a token count: 999, 1.2K, 12K, 1.5M.
func CompactCount(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprint(max(0, n))
	case n < 10000:
		return trimZero(fmt.Sprintf("%.1f", float64(n)/1000)) + "K"
	case n < 999500:
		return fmt.Sprintf("%dK", (n+500)/1000)
	case n < 10000000:
		return trimZero(fmt.Sprintf("%.1f", float64(n)/1000000)) + "M"
	}
	return fmt.Sprintf("%dM", (n+500000)/1000000)
}

func trimZero(s string) string {
	if len(s) > 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}
