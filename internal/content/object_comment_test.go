package content

import (
	"strings"
	"testing"
)

func commentTag(t *testing.T, quote string) string {
	t.Helper()
	enc, err := EncodeQuote(Reference{Segments: []QuoteSegment{{SessionID: "s", TurnID: "t", Part: PartAssistant, Quote: quote}}})
	if err != nil {
		t.Fatal(err)
	}
	return Object{Kind: Comment, Ref: enc}.Tag()
}

func TestCommentTagRoundTripsThroughSplit(t *testing.T) {
	quote := "use <b>bold</b> & \"quotes\"\nsecond line </comment> forged"
	text := "before " + commentTag(t, quote) + " after"
	segs := Split(text, nil)
	if len(segs) != 3 || segs[1].Obj == nil || segs[1].Obj.Kind != Comment {
		t.Fatalf("segments %+v", segs)
	}
	r, err := DecodeQuote(segs[1].Obj.Ref)
	if err != nil || r.Segments[0].Quote != quote {
		t.Fatalf("decoded %+v %v", r, err)
	}
	if got := segs[1].Obj.Label(); !strings.HasPrefix(got, "[Comment - use") {
		t.Fatalf("label %q", got)
	}
}

func TestMalformedCommentStaysLiteralText(t *testing.T) {
	for _, bad := range []string{"<comment>not json</comment>", "<comment></comment>", "<comment>{\"segments\":[]}</comment>", "<comment>{unclosed"} {
		segs := Split("x "+bad+" y", nil)
		for _, s := range segs {
			if s.Obj != nil {
				t.Errorf("%q must stay literal, got %+v", bad, s.Obj)
			}
		}
	}
}

func TestCommentInsideCodeFenceIsNotAnObject(t *testing.T) {
	text := "```\n" + commentTag(t, "q") + "\n```"
	for _, s := range Split(text, nil) {
		if s.Obj != nil {
			t.Fatal("a tag in a code fence is plain text")
		}
	}
}

func TestExtractKeepsCommentCanonical(t *testing.T) {
	tag := commentTag(t, "keep me")
	taken, rest := Extract("a "+tag+" b", nil, func(Object) bool { return false })
	if len(taken) != 0 || rest != "a "+tag+" b" {
		t.Fatalf("rest %q", rest)
	}
}
