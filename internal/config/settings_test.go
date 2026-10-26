package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestLoadSettingsNormalizesInvalidValues(t *testing.T) {
	for _, data := range []string{
		`{"effort":"obsolete","tool":"bash","view":"bad","bash_timeout":"bad"}`,
		`{"effort":"","tool":"bash","view":"","bash_timeout":""}`,
		`{}`, `null`, `{"tool":"bash","view":"loud"}`,
	} {
		t.Run(data, func(t *testing.T) {
			var got Settings
			err := json.Unmarshal([]byte(data), &got)
			if err != nil || got != DefaultSettings() {
				t.Fatalf("load: %+v %v", got, err)
			}
		})
	}
}

func TestSettingsDecodeValidAndLegacy(t *testing.T) {
	for _, view := range []string{"full", "bash_only", "compact"} {
		want := Settings{Provider: "custom", Model: "custom-model", Effort: "max", Tool: "bash", View: view, TurnSummary: "off", Notify: "done", NotifyCommand: "say hi", NotifySound: "chime", NotifyVolume: 60, CompactPercent: 70, CompactKeep: 10, CompactSummaryTokens: 3000, ElidePercent: 60, SystemPrompt: "writer", MaxSpawnDepth: 3, MaxWorkers: 5}
		b, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		var got Settings
		if err := json.Unmarshal(b, &got); err != nil || got != want {
			t.Fatalf("valid round trip: %+v %v", got, err)
		}
	}
	var got Settings
	if err := json.Unmarshal([]byte(`{"model":"legacy","unknown":true,"permissions":"ask"}`), &got); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["permissions"]; exists {
		t.Fatal("obsolete setting persisted")
	}
	want := DefaultSettings()
	want.Model = "legacy"
	if got != want {
		t.Fatalf("legacy defaults: %+v", got)
	}
}

func TestObsoleteSettingsKeysAreIgnored(t *testing.T) {
	data := []byte(`{"bash_timeout":"75","bash_strings":"lines","bash_string_lines":3,"js_timeout":"sec","js_timeout_sec":45,"js_strings":"off","bash_lines":7}`)
	var got Settings
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("%+v %v", got, err)
	}
	out, _ := json.Marshal(got)
	for _, key := range []string{"bash_timeout", "bash_strings", "bash_string_lines", "js_timeout"} {
		if strings.Contains(string(out), key) {
			t.Errorf("obsolete key written back: %s", key)
		}
	}
}

func TestSettingsMigrateOldJSChoices(t *testing.T) {
	var got Settings
	data := []byte(`{"js_code":"full","js_lines":8,"js_output":"on","js_output_lines":9,"js_timing":"slow","thinking":"on","bash_code":"off"}`)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Tool != "js" || got.View != "compact" {
		t.Fatalf("legacy: %+v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"js_code", "js_lines", "js_output", "js_output_lines", "js_timing", "bash_code", "bash_lines", "bash_output", "bash_timing", "thinking", "thinking_lines"} {
		if _, exists := fields[key]; exists {
			t.Errorf("removed key persisted: %s", key)
		}
	}
}

func TestLegacyNotifyQuestionMigrates(t *testing.T) {
	for data, want := range map[string]string{
		`{"notify_question":"on"}`:                "question",
		`{"notify_question":"off"}`:               "off",
		`{"notify":"off","notify_question":"on"}`: "off",
		`{"notify":"done"}`:                       "done",
		`{"notify":"loud"}`:                       "off",
	} {
		var s Settings
		if err := json.Unmarshal([]byte(data), &s); err != nil || s.Notify != want {
			t.Errorf("%s: notify=%q err=%v, want %q", data, s.Notify, err, want)
		}
	}
	b, _ := json.Marshal(DefaultSettings())
	if strings.Contains(string(b), "notify_question") {
		t.Fatal("legacy key written")
	}
}

func TestTurnSummarySetting(t *testing.T) {
	d := DefaultSettings()
	if d.TurnSummary != "on" {
		t.Fatalf("default: %q", d.TurnSummary)
	}
	for _, data := range []string{`{}`, `{"turn_summary":"maybe"}`, `{"turn_summary":""}`} {
		var got Settings
		if err := json.Unmarshal([]byte(data), &got); err != nil || got.TurnSummary != "on" {
			t.Fatalf("%s: %+v %v", data, got, err)
		}
	}
	var got Settings
	if err := json.Unmarshal([]byte(`{"turn_summary":"off"}`), &got); err != nil || got.TurnSummary != "off" {
		t.Fatalf("valid value lost: %+v %v", got, err)
	}
	s := DefaultSettings()
	if s.Cycle("turn_summary", -1) != "off" || s.Cycle("turn_summary", 1) != "on" {
		t.Fatal("turn_summary cycle")
	}
	if s.Set("turn_summary", "") == nil {
		t.Fatal("invalid value accepted")
	}
	if NumOption("turn_summary") != "" {
		t.Fatal("turn_summary carries no number")
	}
}

// The web settings were removed in 0.6.9; an old settings file that still has
// them loads, and the keys are not written back.
func TestRemovedWebSettingsAreIgnored(t *testing.T) {
	var got Settings
	if err := json.Unmarshal([]byte(`{"web_search":"p/m","web_allow_local":true,"effort":"high"}`), &got); err != nil || got.Effort != "high" {
		t.Fatalf("%+v %v", got, err)
	}
	out, _ := json.Marshal(got)
	for _, key := range []string{"web_search", "web_allow_local"} {
		if strings.Contains(string(out), key) {
			t.Errorf("removed key written back: %s", key)
		}
	}
}

func TestElidePercentDefaultsAndRange(t *testing.T) {
	if DefaultSettings().ElidePercent != 50 {
		t.Fatalf("default %d, want 50", DefaultSettings().ElidePercent)
	}
	for _, c := range []struct{ in, want int }{{0, 50}, {-5, 50}, {9, 50}, {10, 10}, {70, 70}, {100, 100}, {101, 50}} {
		var st Settings
		if err := json.Unmarshal([]byte(fmt.Sprintf(`{"elide_percent": %d}`, c.in)), &st); err != nil {
			t.Fatal(err)
		}
		if st.ElidePercent != c.want {
			t.Errorf("elide_percent %d became %d, want %d", c.in, st.ElidePercent, c.want)
		}
	}
	var absent Settings
	if err := json.Unmarshal([]byte(`{}`), &absent); err != nil || absent.ElidePercent != 50 {
		t.Fatalf("a file without the key: %d %v", absent.ElidePercent, err)
	}
}

func TestRemovedImageProtocolIsIgnored(t *testing.T) {
	for _, protocol := range []string{"auto", "off", "kitty", "sixel", "iterm", "unknown"} {
		var got Settings
		if err := json.Unmarshal([]byte(fmt.Sprintf(`{"images_protocol":%q,"view":"full","model":"keep-me"}`, protocol)), &got); err != nil {
			t.Fatal(err)
		}
		want := DefaultSettings()
		want.View, want.Model = "full", "keep-me"
		if got != want {
			t.Fatalf("obsolete protocol changed settings: %+v", got)
		}
		data, err := json.Marshal(got)
		if err != nil || strings.Contains(string(data), "images_protocol") {
			t.Fatalf("obsolete protocol must not be persisted: %s, %v", data, err)
		}
	}
	s := DefaultSettings()
	if Choices("images") != nil || s.Get("images") != "" {
		t.Fatal("obsolete image protocol is still exposed")
	}
	if err := s.Set("images", "kitty"); err == nil {
		t.Fatal("removed protocol setting must not be accepted")
	}
}
