package render

import (
	"container/list"
	"strings"
	"testing"
)

func BenchmarkBashHighlightCached(b *testing.B) {
	src := `if [ -n "$HOME" ]; then printf '%s\n' "${HOME:-/tmp}" | grep tmp; fi`
	bashSegments(src)
	b.ReportAllocs()
	for b.Loop() {
		bashSegments(src)
	}
}
func BenchmarkBashHighlightHeredoc(b *testing.B) {
	src := "cat <<'EOF'\n" + strings.Repeat("$literal value\n", 200) + "EOF\n"
	b.ReportAllocs()
	for b.Loop() {
		bashCache.Lock()
		bashCache.entries = map[string]*list.Element{}
		bashCache.order.Init()
		bashCache.bytes = 0
		bashCache.Unlock()
		bashSegments(src)
	}
}
