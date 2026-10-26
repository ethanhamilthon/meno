package render

import (
	"strings"
	"testing"
)

func TestBashTokensAreLossless(t *testing.T) {
	for _, src := range []string{"", `echo "$HOME" | grep x && printf '%s' ok`, "cat <<'EOF'\n$literal\nEOF\n", "printf '\\n'\r\n", "echo 世界😀\t# comment", "echo \"unfinished", strings.Repeat("x", (256<<10)+1)} {
		var got strings.Builder
		for _, s := range bashSegments(src) {
			got.WriteString(s.Text)
		}
		if got.String() != src {
			t.Fatalf("source changed: %q", src)
		}
	}
}
func TestBashHeaderAndSourceBackgrounds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		turn    Turn
		running bool
	}{{"success", Turn{Code: "echo ok"}, false}, {"running", Turn{Code: "echo ok"}, true}, {"failed", Turn{Code: "false", Err: "exit 1"}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			lines := jsBlock(tc.turn, 80, JSOpts{Mode: JSFull, Running: tc.running})
			if !lines[0].FullWidth || lines[0].BG != BgTerm {
				t.Fatal("header surface")
			}
			if len(lines) < 2 || lines[1].BG != BgTerm {
				t.Fatal("source background")
			}
		})
	}
}
