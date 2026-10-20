package llm

import (
	"strings"
	"testing"
)

func TestToolTitle(t *testing.T) {
	if got := ToolTitle(`{"code":"1","title":"  Read   the\nconfig "}`); got != "Read the config" {
		t.Fatalf("title: %q", got)
	}
	if ToolTitle(`{"code":"1"}`) != "" || ToolTitle("not json") != "" {
		t.Fatal("a missing title must be empty")
	}
	long := ToolTitle(`{"title":"` + strings.Repeat("ж", 200) + `"}`)
	if n := len([]rune(long)); n != MaxTitleRunes {
		t.Fatalf("long title has %d runes", n)
	}
}

func TestToolSchemaAsksForATitle(t *testing.T) {
	p := ToolSchema("")["parameters"].(map[string]any)
	if _, ok := p["properties"].(map[string]any)["title"]; !ok {
		t.Fatal("no title property")
	}
	if _, ok := p["properties"].(map[string]any)["timeout"]; ok {
		t.Fatal("the tool must not offer a timeout")
	}
	req := p["required"].([]string)
	if len(req) != 2 || req[1] != "title" {
		t.Fatalf("required: %v", req)
	}
}
