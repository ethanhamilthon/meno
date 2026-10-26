package render

import (
	"strings"
	"testing"
)

func segText(lines []Line) string {
	var rows []string
	for _, l := range lines {
		var b strings.Builder
		for _, s := range l.Segs {
			b.WriteString(s.Text)
		}
		rows = append(rows, b.String())
	}
	return strings.Join(rows, "\n")
}

func TestBashQuotedContentStaysReadable(t *testing.T) {
	src := `printf '%s' "SELECT * -- <b>html</b> function(){}"`
	lines := CodeLinesOpt(src, BgDiffAdd, 0, 0, CodeOpts{})
	if segText(lines) != src {
		t.Fatalf("source changed: %q", segText(lines))
	}
	var green bool
	for _, seg := range lines[0].Segs {
		green = green || seg.FG == Green
	}
	if !green {
		t.Fatal("shell string was not highlighted")
	}
}

func TestStreamingCodeIsPlain(t *testing.T) {
	plain := CodeLinesOpt("const a = 'x'", BgDiffAdd, 0, 0, CodeOpts{Plain: true})
	if len(plain[0].Segs) != 1 || plain[0].Segs[0].FG != FgWhite {
		t.Fatalf("streaming fragments are not highlighted: %+v", plain[0].Segs)
	}
}
