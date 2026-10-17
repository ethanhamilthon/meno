package app

// vcmd is a parsed normal/visual-mode command.
//
//	[count] [op [count]] key [arg]
//
// key is a motion ("w", "gg", "f"), a text object ("iw", "a\"") after an
// operator or in visual mode, or a standalone command ("x", "p", "dd"→op repeat).
type vcmd struct {
	n      int  // total repeat count (product of both counts), >= 1
	hasN   bool // any count was typed
	op     rune // d c y, or 0
	key    string
	ch     rune // argument of f F t T r
	opLine bool // operator repeated (dd, cc, yy): acts on lines
}

type parseState int

const (
	parseMore parseState = iota // valid prefix, wait for more keys
	parseDone
	parseInvalid
)

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func takeCount(buf []rune, i int) (n int, ok bool, next int) {
	// A leading 0 is the "0" motion, not part of a count.
	if i >= len(buf) || !isDigit(buf[i]) || buf[i] == '0' {
		return 1, false, i
	}
	n = 0
	for i < len(buf) && isDigit(buf[i]) {
		n = n*10 + int(buf[i]-'0')
		if n > 100000 {
			n = 100000
		}
		i++
	}
	return n, true, i
}

// parseVim parses buf. visual enables text objects without an operator.
func parseVim(buf []rune, visual bool) (vcmd, parseState) {
	c := vcmd{n: 1}
	n1, has1, i := takeCount(buf, 0)
	c.n, c.hasN = n1, has1
	if i >= len(buf) {
		return c, parseMore
	}
	k := buf[i]
	i++
	if !visual && (k == 'd' || k == 'c' || k == 'y') {
		c.op = k
		n2, has2, j := takeCount(buf, i)
		i = j
		if has2 {
			c.n, c.hasN = c.n*n2, true
		}
		if i >= len(buf) {
			return c, parseMore
		}
		t := buf[i]
		i++
		if t == k { // dd cc yy
			c.opLine, c.key = true, string(k)
			return c, parseDone
		}
		return parseTarget(c, t, buf[i:], true)
	}
	return parseTarget(c, k, buf[i:], visual)
}

// parseTarget parses a motion/command key and its argument from rest.
func parseTarget(c vcmd, k rune, rest []rune, objects bool) (vcmd, parseState) {
	need := func(prefix string, obj bool) (vcmd, parseState) {
		if len(rest) == 0 {
			return c, parseMore
		}
		if obj {
			c.key = prefix + string(rest[0])
		} else {
			c.key, c.ch = prefix, rest[0]
		}
		return c, parseDone
	}
	switch k {
	case 'g':
		if len(rest) == 0 {
			return c, parseMore
		}
		switch rest[0] {
		case 'g', 'u', 'U', '~', 'J':
			c.key = "g" + string(rest[0])
			return c, parseDone
		}
		return c, parseInvalid
	case 'f', 'F', 't', 'T', 'r':
		return need(string(k), false)
	case 'i', 'a':
		if c.op != 0 || objects {
			return need(string(k), true)
		}
	}
	c.key = string(k)
	return c, parseDone
}
