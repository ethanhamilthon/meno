package render

import (
	"reflect"
	"strings"
	"testing"
)

func TestLimitedOutputMatchesFullReference(t *testing.T) {
	cases := []string{"", "a", "a\n", "a\n\n", "a\r\nb", "\t x\x00", strings.Repeat("row\n", 7), strings.Repeat("界🙂é ", 70), strings.Repeat("x", 1000)}
	for _, text := range cases {
		for _, w := range []int{1, 2, 3, 8, 40, 96} {
			for _, errText := range []bool{false, true} {
				turn := Turn{Output: text}
				if errText {
					turn.Err = text
				}
				want := limitRows(outputLines(turn, w), OutputRows)
				got := limitedTurnOutput(turn, w)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("w=%d error=%v input=%q\ngot=%#v\nwant=%#v", w, errText, text, got, want)
				}
			}
		}
	}
}

func BenchmarkOutputLimit(b *testing.B) {
	for _, n := range []int{100, 100000} {
		b.Run(itoa(n), func(b *testing.B) {
			t := Turn{Code: "echo output", Output: strings.Repeat("some output line\n", n)}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				_ = BuildLines([]Turn{t}, 96, JSOpts{View: ViewFull})
			}
		})
	}
}

func outputLines(t Turn, w int) []Line {
	if t.Err != "" {
		return WrapLines("error: "+t.Err, TranscriptTextWidth(w), func(s string) Line {
			return Plain("  "+s, Red, BgTerm)
		})
	}
	if t.Output == "" {
		return nil
	}
	return WrapLines(t.Output, TranscriptTextWidth(w), func(s string) Line {
		return Plain("  "+s, FgDim, BgTerm)
	})
}
