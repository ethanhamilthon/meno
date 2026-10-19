package cli

import (
	"fmt"
	"sort"
	"strings"
)

func unknown(name string, valid []string) error {
	sorted := append([]string(nil), valid...)
	sort.Strings(sorted)
	message := fmt.Sprintf("unknown command %q; valid names: %s", name, strings.Join(sorted, ", "))
	if hint := closest(name, sorted); hint != "" {
		message = fmt.Sprintf("%s; did you mean %q?", message, hint)
	}
	return fmt.Errorf("%s", message)
}

func closest(word string, options []string) string {
	best, score := "", len([]rune(word))+1
	for _, option := range options {
		d := distance(word, option)
		if d < score {
			best, score = option, d
		}
	}
	if score <= max(1, len([]rune(word))/3) {
		return best
	}
	return ""
}

func distance(a, b string) int {
	x, y := []rune(a), []rune(b)
	row := make([]int, len(y)+1)
	for j := range row {
		row[j] = j
	}
	for i, ca := range x {
		prev := row[0]
		row[0] = i + 1
		for j, cb := range y {
			old := row[j+1]
			cost := 0
			if ca != cb {
				cost = 1
			}
			row[j+1] = min(row[j+1]+1, min(row[j]+1, prev+cost))
			prev = old
		}
	}
	return row[len(y)]
}
