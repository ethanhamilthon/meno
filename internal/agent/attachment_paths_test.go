package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/llm"
)

func TestUserMessageIncludesStoredAttachmentPaths(t *testing.T) {
	s := NewSession(".", "m")
	s.AttachDir = filepath.Join(t.TempDir(), "image files")
	combined := Combine([]Message{
		{Text: "look [Image #1]", Attachments: []Attachment{{N: 1, Mime: "image/png", SHA: "a"}}},
		{Text: "and this", Attachments: []Attachment{{N: 1, Mime: "image/jpeg", SHA: "b"}}},
	})
	turn := Turn{Role: llm.RoleUser, Text: combined.Text, System: "system context", Attachments: combined.Attachments}
	s.Append(turn)
	got := s.ToMsgs()[0]
	want := "system context\n\nlook [Image #1]\n\nand this\n\n" +
		fmt.Sprintf("[Image #1] path: %q\n[Image #2] path: %q", s.AttachmentPath(turn.Attachments[0]), s.AttachmentPath(turn.Attachments[1]))
	if got.Text != want || len(got.Images) != 2 {
		t.Fatalf("message: %+v, want text %q and two images", got, want)
	}
	for i, image := range got.Images {
		if image.N != i+1 || image.Path != s.AttachmentPath(turn.Attachments[i]) || image.Mime != turn.Attachments[i].Mime {
			t.Fatalf("image %d: %+v", i, image)
		}
	}
	if s.Snapshot()[0].Text != turn.Text {
		t.Fatal("model metadata must not change stored user text")
	}
	plain := Turn{Role: llm.RoleUser, Text: "hello", System: "context"}
	if got := s.userMsg(plain); got.Text != withSystem(plain) || len(got.Images) != 0 {
		t.Fatalf("plain message changed: %+v", got)
	}
}

func TestOmittedImageRetainsNumberedPath(t *testing.T) {
	s := NewSession(".", "m")
	s.AttachDir = t.TempDir()
	for i := 0; i <= MaxRequestImages; i++ {
		s.Append(Turn{Role: llm.RoleUser, Text: "look [Image #1]", Attachments: []Attachment{{N: 1, Mime: "image/png", SHA: fmt.Sprint(i)}}})
	}
	got := s.ToMsgs()[0]
	want := fmt.Sprintf("look [image omitted]\n\n[Image #1] path: %q", filepath.Join(s.AttachDir, "0.png"))
	if got.Text != want || len(got.Images) != 0 {
		t.Fatalf("omitted image must keep its numbered file path: %+v", got)
	}
}

func TestAttachmentPathIsUsableWithoutOriginalSource(t *testing.T) {
	s := NewSession(".", "m")
	s.AttachDir = t.TempDir()
	at := Attachment{N: 1, Mime: "image/png", SHA: "stored"}
	path := s.AttachmentPath(at)
	if err := os.WriteFile(path, []byte("stored image bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	got := s.userMsg(Turn{Role: llm.RoleUser, Attachments: []Attachment{at}})
	if !strings.Contains(got.Text, fmt.Sprintf("[Image #1] path: %q", path)) {
		t.Fatalf("missing path: %q", got.Text)
	}
	if data, err := os.ReadFile(got.Images[0].Path); err != nil || string(data) != "stored image bytes" {
		t.Fatalf("stored attachment is not accessible: %q, %v", data, err)
	}
}
