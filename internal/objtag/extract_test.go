package objtag

import (
	"reflect"
	"testing"
)

func isSkill(o Object) bool { return o.Kind == Skill }

func TestExtractTakesOnlySkillsAndKeepsTheOtherObjects(t *testing.T) {
	text := "<skill>goal</skill> read <file-link>/tmp/a b.go</file-link>, <folder-link>/tmp</folder-link> " +
		"<skill>review</skill> <mode>brainstorm</mode>"
	taken, rest := Extract(text, nil, isSkill)
	want := []Object{{Skill, "goal"}, {Skill, "review"}}
	if !reflect.DeepEqual(taken, want) {
		t.Fatalf("taken = %v, want %v", taken, want)
	}
	wantRest := " read <file-link>/tmp/a b.go</file-link>, <folder-link>/tmp</folder-link>  <mode>brainstorm</mode>"
	if rest != wantRest {
		t.Fatalf("rest = %q, want %q", rest, wantRest)
	}
}

func TestExtractKeepsEscapedPathsByteForByte(t *testing.T) {
	text := "<skill>goal</skill><file-link>/a/x&amp;y&lt;z&gt;.go</file-link>"
	taken, rest := Extract(text, nil, isSkill)
	if len(taken) != 1 || rest != "<file-link>/a/x&amp;y&lt;z&gt;.go</file-link>" {
		t.Fatalf("taken = %v, rest = %q", taken, rest)
	}
	if _, again := Extract(rest, nil, nil); again != rest {
		t.Fatalf("round trip changed the text: %q", again)
	}
}

func TestExtractLeavesCodeAndRelativePathsAlone(t *testing.T) {
	bt := "`"
	text := "run " + bt + "<skill>goal</skill>" + bt + " and <file-link>rel.go</file-link>\n" +
		bt + bt + bt + "\n<skill>goal</skill>\n" + bt + bt + bt
	taken, rest := Extract(text, nil, isSkill)
	if len(taken) != 0 || rest != text {
		t.Fatalf("taken = %v, rest = %q", taken, rest)
	}
}

func TestExtractWithNilTakeTakesNothing(t *testing.T) {
	text := "a <file-link>/x</file-link> <skill>goal</skill> b"
	taken, rest := Extract(text, nil, nil)
	if len(taken) != 0 || rest != text {
		t.Fatalf("taken = %v, rest = %q", taken, rest)
	}
}

func TestExtractHonoursTheCheck(t *testing.T) {
	ok := func(k Kind, ref string) bool { return ref == "goal" }
	taken, rest := Extract("<mode>nope</mode> <skill>goal</skill>", ok, isSkill)
	if len(taken) != 1 || taken[0].Ref != "goal" || rest != "<mode>nope</mode> " {
		t.Fatalf("taken = %v, rest = %q", taken, rest)
	}
}
