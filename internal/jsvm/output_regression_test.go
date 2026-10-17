package jsvm

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestVMOutputByteBudget(t *testing.T) {
	for _, limit := range []int{1, 2, 3, 15, 16, 32, 64} {
		for _, code := range []string{
			`console.log("🙂".repeat(100000));`,
			`"🙂".repeat(100000)`,
			`console.log("界".repeat(7)); "🙂".repeat(100000)`,
			`for (let i=0;i<100;i++) console.log("界");`,
		} {
			v := New(Options{MaxOut: limit})
			r := v.Exec(context.Background(), code)
			out := r.Output + r.Value
			if r.Err != "" || len(out) > limit || !utf8.ValidString(r.Output) || !utf8.ValidString(r.Value) || !strings.HasSuffix(out, truncationMarker(limit)) {
				t.Fatalf("limit %d, code %s: %+v (%d bytes)", limit, code, r, len(out))
			}
			if v.out.buf.Len() > limit {
				t.Fatal("retained buffer exceeds limit")
			}
			if r = v.Exec(context.Background(), `1`); r.Err != "" || r.Output != "" || r.Value != "1" {
				t.Fatalf("reuse: %+v", r)
			}
		}
	}
}

func TestVMExactOutputAndReturnedValue(t *testing.T) {
	v := New(Options{MaxOut: 16})
	r := v.Exec(context.Background(), `console.log("1234567"); "12345678"`)
	if r.Output != "1234567\n" || r.Value != "12345678" || r.Err != "" {
		t.Fatalf("exact budget: %+v", r)
	}
	r = v.Exec(context.Background(), `({toString(){throw new Error("conversion failed")}})`)
	if !strings.Contains(r.Err, "conversion failed") {
		t.Fatalf("conversion exception: %+v", r)
	}
	if r = v.Exec(context.Background(), `42`); r.Value != "42" || r.Err != "" {
		t.Fatalf("reuse after exception: %+v", r)
	}
}

func TestBoundedBufferUTF8(t *testing.T) {
	for _, limit := range []int{1, 2, 3, 15, 16, 32, 64} {
		b := boundedBuffer{max: limit}
		input := []byte(strings.Repeat("🙂界", 100))
		// Shell writes may split a multibyte rune across arbitrary chunks.
		for _, c := range input {
			if n, err := b.Write([]byte{c}); n != 1 || err != nil {
				t.Fatal(n, err)
			}
		}
		out := b.String()
		if len(out) > limit || !utf8.ValidString(out) || !strings.HasSuffix(out, truncationMarker(limit)) {
			t.Fatalf("limit %d: %q", limit, out)
		}
	}
	b := boundedBuffer{max: 32}
	b.Write([]byte("hello\xffworld"))
	if out := b.String(); !utf8.ValidString(out) || !strings.Contains(out, "world") {
		t.Fatalf("invalid bytes: %q", out)
	}
}

func TestVMNegativeLimitUsesDefault(t *testing.T) {
	v := New(Options{MaxOut: -1})
	r := v.Exec(context.Background(), `"x".repeat(100000)`)
	if len(r.Value) > 64*1024 || !strings.Contains(r.Value, "truncated") {
		t.Fatalf("negative limit: %d bytes", len(r.Value))
	}
}
