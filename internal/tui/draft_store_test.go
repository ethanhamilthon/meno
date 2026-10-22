package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDraftRoundTripKeepsImageBytes(t *testing.T) {
	d := draftStore{dir: t.TempDir()}
	f := draftFile{Client: "c1", Session: "s1", Text: "hello [Image #1]", Images: []attachment{{N: 1, SHA: "abc"}}}
	if err := d.Save(f, map[string][]byte{"abc": {1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	got, err := d.Load("c1", "s1")
	if err != nil || got.Text != f.Text || len(got.Images) != 1 {
		t.Fatalf("load %+v %v", got, err)
	}
	if b, err := d.Image("abc"); err != nil || len(b) != 3 {
		t.Fatalf("blob %v %v", b, err)
	}
	info, _ := os.Stat(d.path("c1", "s1"))
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatal("draft must be private")
	}
}

func TestTwoWindowsKeepSeparateDrafts(t *testing.T) {
	d := draftStore{dir: t.TempDir()}
	_ = d.Save(draftFile{Client: "a", Session: "s", Text: "from A"}, nil)
	_ = d.Save(draftFile{Client: "b", Session: "s", Text: "from B"}, nil)
	a, _ := d.Load("a", "s")
	b, _ := d.Load("b", "s")
	if a.Text != "from A" || b.Text != "from B" {
		t.Fatalf("a=%q b=%q", a.Text, b.Text)
	}
	if n := len(d.Recoverable("a")); n != 1 {
		t.Fatalf("recoverable for a: %d", n)
	}
}

func TestEmptyDraftRemovesFileAndCorruptIsSkipped(t *testing.T) {
	d := draftStore{dir: t.TempDir()}
	_ = d.Save(draftFile{Client: "a", Session: "s", Text: "x"}, nil)
	_ = d.Save(draftFile{Client: "a", Session: "s"}, nil)
	if _, err := os.Stat(d.path("a", "s")); !os.IsNotExist(err) {
		t.Fatal("empty draft must remove the file")
	}
	_ = os.WriteFile(filepath.Join(d.dir, "x-y.json"), []byte("{broken"), 0o600)
	if n := len(d.Recoverable("me")); n != 0 {
		t.Fatalf("corrupt file listed: %d", n)
	}
}

func TestNewerDraftVersionIsNotListed(t *testing.T) {
	d := draftStore{dir: t.TempDir()}
	_ = os.WriteFile(filepath.Join(d.dir, "x-y.json"), []byte(`{"version":99,"client":"z","text":"future"}`), 0o600)
	if n := len(d.Recoverable("me")); n != 0 {
		t.Fatalf("future version listed: %d", n)
	}
}
