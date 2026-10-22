package content

import "testing"

func texts(segs []Segment) []string {
	var out []string
	for _, s := range segs {
		if s.Obj != nil {
			out = append(out, "["+string(s.Obj.Kind)+":"+s.Obj.Ref+"]")
		} else {
			out = append(out, s.Text)
		}
	}
	return out
}

func TestSplitAcceptsOnlyWellFormedTags(t *testing.T) {
	ok := func(Kind, string) bool { return true }
	cases := map[string][]string{
		"see <file-link>/a/b c.go</file-link> now":  {"see ", "[file:/a/b c.go]", " now"},
		"<folder-link>/tmp</folder-link>":           {"[folder:/tmp]"},
		"use <skill>review</skill>":                 {"use ", "[skill:review]"},
		"<file-link>relative.go</file-link>":        {"<file-link>relative.go</file-link>"},
		"<file-link>/a</file-link":                  {"<file-link>/a</file-link"},
		"<file-link>/a\nb</file-link>":              {"<file-link>/a\nb</file-link>"},
		"<file-link></file-link>":                   {"<file-link></file-link>"},
		"/plain/path and /skill stay text":          {"/plain/path and /skill stay text"},
		"code ¤<file-link>/x</file-link>¤ here":     {"code ¤<file-link>/x</file-link>¤ here"},
		"¤¤¤\n<file-link>/x</file-link>\n¤¤¤":       {"¤¤¤\n<file-link>/x</file-link>\n¤¤¤"},
		"<file-link>/a&amp;b&lt;c&gt;</file-link>":  {"[file:/a&b<c>]"},
		"<file-link>/a</file-link><skill>s</skill>": {"[file:/a]", "[skill:s]"},
	}
	for in, want := range cases {
		in2 := replaceTick(in)
		got := texts(Split(in2, ok))
		w := make([]string, len(want))
		for i := range want {
			w[i] = replaceTick(want[i])
		}
		if len(got) != len(w) {
			t.Errorf("%q => %q, want %q", in2, got, w)
			continue
		}
		for i := range got {
			if got[i] != w[i] {
				t.Errorf("%q => %q, want %q", in2, got, w)
				break
			}
		}
	}
	if segs := Split("<skill>nope</skill>", func(_ Kind, n string) bool { return n == "yes" }); segs[0].Obj != nil {
		t.Fatal("an unavailable skill stays text")
	}
	if segs := Split("<mode>goal</mode>", nil); len(segs) != 1 || segs[0].Obj != nil {
		t.Fatal("removed modes must stay literal")
	}

}

func replaceTick(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r == '¤' {
			out[i] = '`'
		}
	}
	return string(out)
}

func TestTagEscapingRoundTrips(t *testing.T) {
	o := Object{File, "/weird/<name> & \"x\".go"}
	segs := Split("a "+o.Tag()+" b", nil)
	if len(segs) != 3 || segs[1].Obj == nil || *segs[1].Obj != o {
		t.Fatalf("%v", texts(segs))
	}
	if o.Label() != "@<name> & \"x\".go" {
		t.Fatalf("label %q", o.Label())
	}
}
