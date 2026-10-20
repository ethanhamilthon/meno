package ui

func Pad(s string, w int) string {
	n := TextWidth(s)
	if n >= w {
		return s
	}
	out := s
	for i := 0; i < w-n; i++ {
		out += " "
	}
	return out
}

func Truncate(s string, w int) string {
	if TextWidth(s) <= w {
		return s
	}
	if w <= 1 {
		return ""
	}
	r := []rune(s)
	for len(r) > 0 && TextWidth(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
