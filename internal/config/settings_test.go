package config

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLoadSettingsNormalizesInvalidValues(t *testing.T) {
	for _, data := range []string{
		`{"effort":"obsolete","js_code":"bad","js_output":"bad","thinking":"bad","js_timeout":"bad","js_lines":-1}`,
		`{"effort":"","js_code":"","js_output":"","thinking":"","js_timeout":"","js_lines":51}`,
		`{}`, `null`, `{"js_lines":0}`,
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
		want := Settings{Provider: "custom", Model: "custom-model", Effort: "max", JSCode: "off", JSLines: lines, JSOutLines: 7, JSOutput: "off", JSStrings: "off", JSStringLines: 2, Thinking: "off", ThinkLines: 4, JSTiming: "slow", TurnSummary: "off", JSTimeout: "model", JSTimeoutSec: 90, Theme: "dracula", Notify: "done", NotifyCommand: "say hi", NotifySound: "chime", NotifyVolume: 60, WebSearch: "brave", WebSearchKey: "key", SubagentProvider: "openai", SubagentModel: "gpt-5", SubagentEffort: "low", CompactPercent: 70, CompactKeep: 10, CompactSummaryTokens: 3000}
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

func TestMalformedSettingsDoNotPartiallyApply(t *testing.T) {
	for _, data := range []string{`{"model":"changed","js_lines":"bad"}`, `{"js_timeout":3}`, `{"js_lines":999999999999999999999999}`, `{"model":`} {
		got := DefaultSettings()
		err := json.Unmarshal([]byte(data), &got)
		if err == nil || got != DefaultSettings() {
			t.Fatalf("malformed settings applied: %+v %v", got, err)
		}
	}
}

func TestUnknownTimeoutIsBounded(t *testing.T) {
	for _, value := range []string{"", "typo", "30s"} {
		if got := (Settings{JSTimeout: value}).TimeoutDuration(); got != 30*time.Second {
			t.Fatalf("%q: %s", value, got)
		}
	}
}

func TestSettingsMigrateOldJSChoices(t *testing.T) {
	var got Settings
	if err := json.Unmarshal([]byte(`{"js_code":"full","js_output":"on","thinking":"on","js_timeout":"5s"}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.JSCode != "all" || got.JSOutput != "all" || got.JSOutLines != 5 || got.Thinking != "all" || got.ThinkLines != 5 || got.JSTimeout != "sec" || got.JSTimeoutSec != 5 {
		t.Fatalf("%+v", got)
	}
	got = Settings{}
	if err := json.Unmarshal([]byte(`{"js_timeout":"none"}`), &got); err != nil || got.JSTimeout != "off" || got.TimeoutDuration() != 0 {
		t.Fatalf("none: %+v %v", got, err)
	}
	got = Settings{}
	if err := json.Unmarshal([]byte(`{"js_timeout":"30s"}`), &got); err != nil || got.JSTimeout != "sec" || got.JSTimeoutSec != 30 {
		t.Fatalf("30s: %+v %v", got, err)
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
	if d.JSTiming != "all" || d.TurnSummary != "on" {
		t.Fatalf("defaults: %q %q", d.JSTiming, d.TurnSummary)
	}
	// Files written before these settings existed, and invalid values, get the defaults.
	for _, data := range []string{`{}`, `{"js_timing":"sometimes","turn_summary":"maybe"}`, `{"js_timing":"","turn_summary":""}`} {
		var got Settings
		if err := json.Unmarshal([]byte(data), &got); err != nil || got.JSTiming != "all" || got.TurnSummary != "on" {
			t.Fatalf("%s: %+v %v", data, got, err)
		}
	}
	var got Settings
	if err := json.Unmarshal([]byte(`{"js_timing":"slow","turn_summary":"off"}`), &got); err != nil || got.JSTiming != "slow" || got.TurnSummary != "off" {
		t.Fatalf("valid values lost: %+v %v", got, err)
	}
	s := DefaultSettings()
	var seen []string
	for i := 0; i < 3; i++ {
		seen = append(seen, s.Cycle("js_timing", 1))
	}
	if strings.Join(seen, ",") != "slow,off,all" {
		t.Fatalf("cycle: %v", seen)
	}
	if s.Cycle("turn_summary", -1) != "off" || s.Cycle("turn_summary", 1) != "on" {
		t.Fatal("turn_summary cycle")
	}
	if s.Set("js_timing", "bogus") == nil || s.Set("turn_summary", "") == nil {
		t.Fatal("invalid values accepted")
	}
	if NumOption("js_timing") != "" || NumOption("turn_summary") != "" {
		t.Fatal("timing settings carry no number")
	}
}
