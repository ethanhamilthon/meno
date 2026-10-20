package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLoadSettingsNormalizesInvalidValues(t *testing.T) {
	for _, data := range []string{
		`{"effort":"obsolete","tool":"bash","bash_code":"bad","bash_output":"bad","thinking":"bad","bash_timeout":"bad","bash_lines":-1}`,
		`{"effort":"","tool":"bash","bash_code":"","bash_output":"","thinking":"","bash_timeout":"","bash_lines":51}`,
		`{}`, `null`, `{"tool":"bash","bash_lines":0}`,
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
	for _, lines := range []int{1, 50} {
		want := Settings{Provider: "custom", Model: "custom-model", Effort: "max", Tool: "bash", BashCode: "off", BashLines: lines, BashOutput: "off", BashOutputLines: 7, BashStrings: "off", BashStringLines: 2, BashTiming: "slow", BashTimeout: "model", TurnSummary: "off", Thinking: "off", ThinkLines: 4, DaemonIdleExitMin: 30, Theme: "dracula", Notify: "done", NotifyCommand: "say hi", NotifySound: "chime", NotifyVolume: 60, CompactPercent: 70, CompactKeep: 10, CompactSummaryTokens: 3000}
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

func TestUnknownTimeoutIsBounded(t *testing.T) {
	for _, value := range []string{"", "typo", "30s"} {
		if got := (Settings{BashTimeout: value}).BashTimeoutDuration(); got != DefaultBashTimeout {
			t.Fatalf("%q: %s", value, got)
		}
	}
}

func TestSettingsMigrateOldJSChoices(t *testing.T) {
	var got Settings
	data := []byte(`{"js_code":"full","js_lines":8,"js_output":"on","js_output_lines":9,"js_strings":"lines","js_string_lines":2,"js_timing":"slow","js_timeout":"5s","thinking":"on"}`)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.BashCode != "all" || got.BashLines != 8 || got.BashOutput != "all" || got.BashOutputLines != 9 || got.BashStrings != "lines" || got.BashStringLines != 2 || got.BashTiming != "slow" || got.BashTimeout != "5" || got.Thinking != "all" || got.Tool != "js" {
		t.Fatalf("legacy aliases: %+v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"js_code", "js_lines", "js_output", "js_output_lines", "js_strings", "js_string_lines", "js_timing", "js_timeout", "js_timeout_sec"} {
		if _, exists := fields[key]; exists {
			t.Errorf("legacy key persisted: %s", key)
		}
	}
}

func TestBashTimeoutMigrationAliases(t *testing.T) {
	for input, want := range map[string]string{
		`{"js_timeout":"sec","js_timeout_sec":45}`:                     "45",
		`{"js_timeout":"none"}`:                                        "off",
		`{"js_timeout":"model"}`:                                       "model",
		`{"js_timeout":"30s"}`:                                         "30",
		`{"js_timeout":"sec","js_timeout_sec":0}`:                      "120",
		`{"bash_timeout":"75","js_timeout":"sec","js_timeout_sec":45}`: "75",
	} {
		var got Settings
		if err := json.Unmarshal([]byte(input), &got); err != nil || got.BashTimeout != want {
			t.Errorf("%s: timeout %q err %v, want %q", input, got.BashTimeout, err, want)
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

func TestTimingSettings(t *testing.T) {
	d := DefaultSettings()
	if d.BashTiming != "all" || d.TurnSummary != "on" {
		t.Fatalf("defaults: %q %q", d.BashTiming, d.TurnSummary)
	}
	// Files written before these settings existed, and invalid values, get the defaults.
	for _, data := range []string{`{}`, `{"bash_timing":"sometimes","turn_summary":"maybe"}`, `{"bash_timing":"","turn_summary":""}`} {
		var got Settings
		if err := json.Unmarshal([]byte(data), &got); err != nil || got.BashTiming != "all" || got.TurnSummary != "on" {
			t.Fatalf("%s: %+v %v", data, got, err)
		}
	}
	var got Settings
	if err := json.Unmarshal([]byte(`{"bash_timing":"slow","turn_summary":"off"}`), &got); err != nil || got.BashTiming != "slow" || got.TurnSummary != "off" {
		t.Fatalf("valid values lost: %+v %v", got, err)
	}
	s := DefaultSettings()
	var seen []string
	for i := 0; i < 3; i++ {
		seen = append(seen, s.Cycle("bash_timing", 1))
	}
	if strings.Join(seen, ",") != "slow,off,all" {
		t.Fatalf("cycle: %v", seen)
	}
	if s.Cycle("turn_summary", -1) != "off" || s.Cycle("turn_summary", 1) != "on" {
		t.Fatal("turn_summary cycle")
	}
	if s.Set("bash_timing", "bogus") == nil || s.Set("turn_summary", "") == nil {
		t.Fatal("invalid values accepted")
	}
	if NumOption("bash_timing") != "" || NumOption("turn_summary") != "" {
		t.Fatal("timing settings carry no number")
	}
}
