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
	"testing"
)

func TestAttachedImagePathCanBeUsedByBash(t *testing.T) {
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
			if f.Requests() != 2 {
				t.Fatalf("requests: got %d, want 2", f.Requests())
			}
		})
	}
}
