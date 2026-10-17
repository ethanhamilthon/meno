package config

import (
	"encoding/json"
	"os"
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
			st := NewStore(Paths{Home: t.TempDir()})
			if err := os.WriteFile(st.P.File("settings.json"), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := st.LoadSettings()
			if err != nil || got != DefaultSettings() {
				t.Fatalf("load: %+v %v", got, err)
			}
		})
	}
}

func TestSettingsDecodeValidAndLegacy(t *testing.T) {
	for _, lines := range []int{1, 50} {
		want := Settings{Provider: "custom", Model: "custom-model", Effort: "max", JSCode: "off", JSLines: lines, JSOutLines: 7, JSOutput: "off", Thinking: "off", ThinkLines: 4, JSTimeout: "model", JSTimeoutSec: 90, Theme: "dracula"}
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
		st := NewStore(Paths{Home: t.TempDir()})
		if err := os.WriteFile(st.P.File("settings.json"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := st.LoadSettings()
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
