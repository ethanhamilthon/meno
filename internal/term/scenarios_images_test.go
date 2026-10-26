//go:build termtest

package term

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestAttachedImagePathCanBeUsedByBash(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "1")
	for _, proto := range allProtocols {
		t.Run(proto, func(t *testing.T) {
			f := newFakeLLM(t)
			pathLine := regexp.MustCompile(`\[Image #1\] path: ("[^"\n]+")`)
			f.Handler = func(q *llmReq) {
				if q.N > 1 {
					if q.Mentions("stored-image-readable") {
						q.Text("attachment-path-verified")
					} else {
						q.Text("attachment-path-failed")
					}
					return
				}
				if !q.Mentions("data:image/png;base64,") {
					q.Text("missing-image-payload")
					return
				}
				// Decode JSON so the assertion uses exactly the text seen by the model.
				var body any
				if err := json.Unmarshal([]byte(q.Body), &body); err != nil {
					q.Text("invalid-request")
					return
				}
				var findText func(any) string
				findText = func(v any) string {
					switch v := v.(type) {
					case string:
						if pathLine.MatchString(v) {
							return v
						}
					case []any:
						for _, item := range v {
							if text := findText(item); text != "" {
								return text
							}
						}
					case map[string]any:
						for _, item := range v {
							if text := findText(item); text != "" {
								return text
							}
						}
					}
					return ""
				}
				match := pathLine.FindStringSubmatch(findText(body))
				if len(match) != 2 {
					q.Text("missing-attachment-path")
					return
				}
				path, err := strconv.Unquote(match[1])
				if err != nil || !filepath.IsAbs(path) {
					q.Text("invalid-attachment-path")
					return
				}
				// The fake provider uses the received path in a real Bash tool call.
				q.Tool("", fmt.Sprintf("test -s %q && echo stored-image-readable", path))
			}
			s := start(t, options{fake: f, proto: proto})
			var data bytes.Buffer
			if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(s.work, "source image.png")
			if err := os.WriteFile(source, data.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			s.Send("\x1b[200~" + strconv.Quote(source) + "\x1b[201~")
			s.WaitFor("[Image #1]")
			if err := os.Remove(source); err != nil {
				t.Fatal(err)
			}
			s.Send(Enter)
			s.WaitFor("attachment-path-verified")
			assertNoTerminalImages(t, s)
			if f.Requests() != 2 {
				t.Fatalf("requests: got %d, want 2", f.Requests())
			}
		})
	}
}

func TestViewDeliversImageWithoutTerminalImageRows(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "1")
	for _, proto := range allProtocols {
		t.Run(proto, func(t *testing.T) {
			f := newFakeLLM(t)
			var source string
			f.Handler = func(q *llmReq) {
				if q.N == 1 {
					q.Tool("", fmt.Sprintf("meno view %q", source))
					return
				}
				if !q.Mentions("data:image/png;base64,") || !q.Mentions("[Image #1] path:") || !q.Mentions("image displayed") {
					q.Text("view-image-missing")
					return
				}
				q.Text("view-image-verified")
			}
			s := start(t, options{fake: f, proto: proto})
			var data bytes.Buffer
			if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
				t.Fatal(err)
			}
			source = filepath.Join(s.work, "view image.png")
			if err := os.WriteFile(source, data.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			s.Say("inspect the image")
			screen := s.WaitFor("view-image-verified")
			if !strings.Contains(screen, "image displayed") || strings.Contains(screen, "Image from view(") || strings.Contains(screen, "▣ Image") {
				t.Fatalf("view must show only ordinary tool output: %s", screen)
			}
			assertNoTerminalImages(t, s)
			if f.Requests() != 2 {
				t.Fatalf("requests: got %d, want 2", f.Requests())
			}
		})
	}
}

func assertNoTerminalImages(t *testing.T, s *session) {
	t.Helper()
	raw := s.RawSince(0)
	for _, marker := range []string{"\x1b_G", "\x1bPq", "\x1b]1337;File="} {
		if bytes.Contains(raw, []byte(marker)) {
			t.Fatalf("terminal image protocol emitted: %q", marker)
		}
	}
}
