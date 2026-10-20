package llm

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// PartialCode extracts the value of the "code" string from a possibly
// incomplete JSON arguments object, e.g. {"code":"const x = 1;\nprint(x"
// yields "const x = 1;\nprint(x". It never fails: incomplete escapes at the
// end are simply left out until more input arrives.
func PartialCode(args string) string {
	if v := partialString(args, "code"); v != "" {
		return v
	}
	return partialString(args, "command")
}

// partialString is PartialCode for one named key.
func partialString(args, key string) string {
	quoted := `"` + key + `"`
	i := strings.Index(args, quoted)
	if i < 0 {
		return ""
	}
	rest := strings.TrimLeft(args[i+len(quoted):], " \t\r\n")
	if !strings.HasPrefix(rest, ":") {
		return ""
	}
	rest = strings.TrimLeft(rest[1:], " \t\r\n")
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	rest = rest[1:]
	var b strings.Builder
	for len(rest) > 0 {
		c := rest[0]
		switch {
		case c == '"':
			return b.String()
		case c != '\\':
			r, n := utf8.DecodeRuneInString(rest)
			if r == utf8.RuneError && n <= 1 && !utf8.FullRuneInString(rest) {
				return b.String()
			}
			b.WriteRune(r)
			rest = rest[n:]
		default:
			if len(rest) < 2 {
				return b.String()
			}
			switch rest[1] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'u':
				if len(rest) < 6 {
					return b.String()
				}
				v, err := strconv.ParseUint(rest[2:6], 16, 32)
				if err != nil {
					return b.String()
				}
				r, n := rune(v), 6
				if r >= 0xD800 && r < 0xDC00 { // surrogate pair
					if len(rest) < 12 || rest[6] != '\\' || rest[7] != 'u' {
						return b.String()
					}
					lo, err := strconv.ParseUint(rest[8:12], 16, 32)
					if err != nil {
						return b.String()
					}
					r = 0x10000 + (r-0xD800)<<10 + (rune(lo) - 0xDC00)
					n = 12
				}
				b.WriteRune(r)
				rest = rest[n:]
				continue
			default:
				b.WriteByte(rest[1])
			}
			rest = rest[2:]
		}
	}
	return b.String()
}
