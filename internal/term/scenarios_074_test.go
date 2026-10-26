//go:build termtest

package term

import (
	"encoding/json"
	"meno/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestComposerPanelsAndLivePalette(t *testing.T) {
	f := newFakeLLM(t)
	s := start(t, options{fake: f, cols: 110, rows: 32})
	s.WaitFor("Keep your agent simple.")
	s.Cmd("settings")
	s.WaitFor("system prompt")
	if strings.Contains(s.Screen(), "General") || strings.Contains(s.Screen(), "theme") {
		t.Fatal("tabs or theme selector remain")
	}
	first := s.RawLen()
	colors := config.DefaultThemeColors()
	colors.Blue = "FF0000"
	raw, _ := json.Marshal(colors)
	if err := os.WriteFile(filepath.Join(s.home, "theme.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for !strings.Contains(string(s.RawSince(first)), "255;0;0") {
		if time.Now().After(deadline) {
			t.Fatal("palette edit not painted while panel open")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if fullClear.Match(s.RawSince(first)) {
		t.Fatal("palette reload cleared terminal")
	}
	s.Send(Esc)
	s.WaitGone("system prompt")
	s.Cmd("model")
	s.WaitFor("↑/↓ model")
	s.Send(Esc)
	s.WaitGone("↑/↓ model")
	s.Type("draft survives ")
	s.Cmd("sessions")
	s.WaitFor("tab filter")
	s.Send(Esc)
	s.WaitGone("tab filter")
	s.WaitFor("draft survives")
}
