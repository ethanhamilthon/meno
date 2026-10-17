package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserMessageWithImages(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.png")
	if err := os.WriteFile(p, []byte("IMG"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := Msg{Role: RoleUser, Text: "see [Image #1] please", Images: []Image{{N: 1, Mime: "image/png", Path: p}, {N: 2, Mime: "image/png", Path: p}}}

	chat, _ := json.Marshal(chatMessages("sys", []Msg{m})[1])
	want := `{"content":[{"text":"see [Image #1]","type":"text"},{"image_url":{"url":"data:image/png;base64,SU1H"},"type":"image_url"},{"text":" please","type":"text"},{"image_url":{"url":"data:image/png;base64,SU1H"},"type":"image_url"}],"role":"user"}`
	if string(chat) != want {
		t.Fatalf("chat:\n%s\nwant:\n%s", chat, want)
	}

	resp, _ := json.Marshal(responsesInput([]Msg{m}))
	if !strings.Contains(string(resp), `{"image_url":"data:image/png;base64,SU1H","type":"input_image"}`) || !strings.Contains(string(resp), `"type":"input_text"`) {
		t.Fatalf("responses: %s", resp)
	}
}

func TestUserMessageWithoutImagesStaysAString(t *testing.T) {
	got := chatMessages("s", []Msg{{Role: RoleUser, Text: "hi [Image #1]"}})[1]["content"]
	if got != "hi [Image #1]" {
		t.Fatalf("%#v", got)
	}
}

func TestMissingImageFileBecomesNote(t *testing.T) {
	m := Msg{Role: RoleUser, Text: "x [Image #1]", Images: []Image{{N: 1, Mime: "image/png", Path: "/nonexistent"}}}
	b, _ := json.Marshal(chatMessages("s", []Msg{m})[1])
	if !strings.Contains(string(b), "[image unavailable]") {
		t.Fatalf("%s", b)
	}
}

func TestDecodeVision(t *testing.T) {
	or, err := decodeOpenRouter(strings.NewReader(`{"data":[{"id":"a/v","context_length":10,"pricing":{"prompt":"0.1","completion":"0.2"},"architecture":{"input_modalities":["text","image"]}},{"id":"a/t","context_length":10,"pricing":{"prompt":"0.1","completion":"0.2"},"architecture":{"input_modalities":["text"]}},{"id":"a/u","context_length":10}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if v := or["openrouter/a/v"].Vision; v == nil || !*v {
		t.Fatal("vision model")
	}
	if v := or["openrouter/a/t"].Vision; v == nil || *v {
		t.Fatal("text model")
	}
	if or["openrouter/a/u"].Vision != nil {
		t.Fatal("unknown must stay nil")
	}
	ll, err := decodeLiteLLM(strings.NewReader(`{"gpt-x":{"litellm_provider":"openai","mode":"chat","max_input_tokens":5,"input_cost_per_token":1e-6,"output_cost_per_token":1e-6,"supports_vision":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	if v := ll["openai/gpt-x"].Vision; v == nil || !*v {
		t.Fatalf("litellm: %+v", ll)
	}
}
