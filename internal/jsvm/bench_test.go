package jsvm

import (
	"context"
	"testing"
)

// BenchmarkFreshRuntimeCall is the cost of one trivial call: a new runtime,
// all bindings and the prelude. Every tool call pays it.
func BenchmarkFreshRuntimeCall(b *testing.B) {
	v := New(Options{Root: b.TempDir()})
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if r := v.Exec(ctx, "1+1"); r.Err != "" {
			b.Fatal(r.Err)
		}
	}
}
