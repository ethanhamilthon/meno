package tui

import (
	"reflect"
	"strings"
	"testing"

	"meno/internal/api"
	"meno/internal/render"
)

func TestAttachmentsHaveNoGeneratedTerminalRepresentation(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	user := api.Turn{ID: "user", Role: render.RoleUser, Content: "look [Image #1]", Attachments: []api.TurnAttachment{{N: 1, Mime: "image/png", SHA: "stored", W: 160, H: 160}}}
	view := api.Turn{ID: "view", Role: render.RoleUser, Kind: render.KindView, Content: "Image from view(\"x.png\"): [Image #1]", Attachments: user.Attachments}
	for _, mode := range []string{"full", "bash_only", "compact"} {
		a.st.Settings.View = mode
		a.st.Session.turns = []render.Turn{turnFromWire(user), turnFromWire(view)}
		got := a.logText()
		plain := user
		plain.ID = "plain"
		plain.Attachments = nil
		a.st.Session.turns = []render.Turn{turnFromWire(plain)}
		if !reflect.DeepEqual(got, a.logText()) || !strings.Contains(string(got.flat()), user.Content) {
			t.Fatalf("%s: attachments must not add captions or rows: %q", mode, got.flat())
		}
	}
	for _, row := range settingRows {
		if row.key == "images" {
			t.Fatal("terminal image protocol setting must be removed")
		}
	}
}
