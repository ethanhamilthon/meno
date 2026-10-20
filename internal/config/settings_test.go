package config

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEffortRouterSettings(t *testing.T) {
	s := DefaultSettings()
	if s.EffortRouter != "off" || s.EffortRouterModel != "" || s.EffortRouterProvider != "" {
		t.Fatalf("defaults: %+v", s)
	}
	for key, value := range map[string]string{"effort_router": "auto", "effort_router_model": " small ", "effort_router_provider": " router "} {
		if err := s.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if s.Get("effort_router") != "auto" || s.Get("effort_router_model") != "small" || s.Get("effort_router_provider") != "router" {
		t.Fatalf("settings: %+v", s)
	}
	if !TextSetting("effort_router_model") || !TextSetting("effort_router_provider") {
		t.Fatal("router model and provider must be text settings")
	}
	if got := s.Cycle("effort_router", 1); got != "off" {
		t.Fatalf("cycle = %q", got)
	}
}

func TestEffortRouterSettingsListed(t *testing.T) {
	for _, key := range []string{"effort_router", "effort_router_model", "effort_router_provider"} {
		found := false
		for _, candidate := range SettingKeys() {
			found = found || candidate == key
		}
		if !found {
			t.Fatalf("SettingKeys() does not include %q", key)
		}
	}
	choices := Choices("effort_router")
	if len(choices) != 2 || choices[0] != "off" || choices[1] != "auto" {
		t.Fatalf("choices: %v", choices)
	}
}

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
		want := Settings{Provider: "custom", Model: "custom-model", Effort: "max", Tool: "js", JSCode: "off", JSLines: lines, JSOutLines: 7, JSOutput: "off", JSStrings: "off", JSStringLines: 2, Thinking: "off", ThinkLines: 4, JSTiming: "slow", TurnSummary: "off", JSTimeout: "model", JSTimeoutSec: 90, BashCode: "off", BashLines: lines, BashOutput: "off", BashOutputLines: 7, BashStrings: "off", BashStringLines: 2, BashTiming: "slow", BashTimeout: "model", DaemonIdleExitMin: 30, Theme: "dracula", Notify: "done", NotifyCommand: "say hi", NotifySound: "chime", NotifyVolume: 60, EffortRouter: "off", WebSearch: "brave", WebSearchKey: "key", SubagentProvider: "openai", SubagentModel: "gpt-5", SubagentEffort: "low", SubagentMaxSpawn: 50, CompactPercent: 70, CompactKeep: 10, CompactSummaryTokens: 3000}
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
	for _, data := range []string{`{"model":"changed","bash_lines":"bad"}`, `{"bash_timeout":3}`, `{"bash_lines":999999999999999999999999}`, `{"model":`} {
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
	if got.JSCode != "all" || got.JSOutput != "all" || got.JSOutLines != 5 || got.Thinking != "all" || got.ThinkLines != 5 || got.JSTimeout != "sec" || got.JSTimeoutSec != 5 || got.Tool != "js" {
		t.Fatalf("%+v", got)
	}
	got = Settings{}
	if err := json.Unmarshal([]byte(`{"js_timeout":"none"}`), &got); err != nil || got.JSTimeout != "off" || got.TimeoutDuration() != 0 || got.Tool != "js" {
		t.Fatalf("none: %+v %v", got, err)
	}
	got = Settings{}
	if err := json.Unmarshal([]byte(`{"js_timeout":"30s"}`), &got); err != nil || got.JSTimeout != "sec" || got.JSTimeoutSec != 30 || got.Tool != "js" {
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
