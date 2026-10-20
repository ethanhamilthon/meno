package app

import (
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/llm"
)

func TestJSStringsSettingFoldsTheTimeline(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Code: "fs.write('a.txt', `one\ntwo\nthree\nfour\nfive`)"})
	if out, _ := transcriptText(a); !strings.Contains(out, "five") {
		t.Fatalf("folding is off by default:\n%s", out)
	}
	a.openOverlay(OverSettings)
	a.st.Sel = settingIdx(t, "bash_strings")
	a.onKey(key(' '))
	if a.st.Settings.BashStrings != "lines" || !a.jsOpts().FoldStrings || a.jsOpts().FoldLines != 3 {
		t.Fatalf("setting not applied: %+v", a.st.Settings)
	}
	out, _ := transcriptText(a)
	if !strings.Contains(out, "three") || strings.Contains(out, "four") || !strings.Contains(out, "…") {
		t.Fatalf("string not folded to 3 lines:\n%s", out)
	}
	a.onKey(key(' '))
	if out, _ := transcriptText(a); !strings.Contains(out, "five") {
		t.Fatalf("string not expanded again:\n%s", out)
	}
}

func TestStringsCommandIsGone(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	for _, c := range commandList {
		if c.name == ":strings" {
			t.Fatal(":strings is still listed")
		}
	}
	a.runCommand("strings")
	a.draw()
	if out := screenText(s); !strings.Contains(out, "unknown command :strings") {
		t.Fatalf("expected unknown command:\n%s", out)
	}
}
