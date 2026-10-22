package tui

import (
	"strings"
	"testing"
)

func TestInputWordWrapPreservesOffsets(t *testing.T) {
	for _, text := range []string{"alpha beta gamma", "  alpha  beta\tmore", "привет снова мир", "abc\n\nend\n", "界界界 abc", "longlonglonglongword"} {
		in := []rune(text)
		rows := layoutInput(in, 13)
		var out strings.Builder
		for i, row := range rows {
			if i > 0 && !rows[i-1].soft {
				out.WriteByte('\n')
			}
			out.WriteString(string(in[row.start:row.end]))
		}
		if out.String() != text {
			t.Fatalf("lost text %q -> %q", text, out.String())
		}
	}
	in := []rune("alpha beta gamma")
	rows := layoutInput(in, 13)
	if string(in[rows[0].start:rows[0].end]) != "alpha beta " {
		t.Fatalf("word split %+v", rows)
	}
}
