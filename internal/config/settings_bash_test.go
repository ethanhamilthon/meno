package config

import (
	"encoding/json"
	"testing"
)

func TestBashDefaultsAliasesAndSettingWrite(t *testing.T) {
	d := DefaultSettings()
	if d.Tool != "bash" {
		t.Fatalf("defaults: %+v", d)
	}
	var legacy Settings
	if err := json.Unmarshal([]byte("{\"js_code\":\"lines\"}"), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Tool != "js" || legacy.View != "compact" {
		t.Fatalf("alias: %+v", legacy)
	}
	for key, value := range map[string]string{"tool": "bash"} {
		if err := d.Set(key, value); err != nil {
			t.Errorf("%s: %v", key, err)
		}
	}
	if d.Tool != "bash" {
		t.Fatalf("set: %+v", d)
	}
	if d.Cycle("tool", -1) != "js" {
		t.Fatal("tool cycle")
	}
	for key, value := range map[string]string{"daemon_idle_exit_min": "0", "theme": "panel-dark"} {
		if err := d.Set(key, value); err == nil {
			t.Errorf("accepted %s=%s", key, value)
		}
	}
	legacyData, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(legacyData, &fields); err != nil {
		t.Fatal(err)
	}
	for _, old := range []string{"js_code", "js_lines", "js_output", "js_output_lines", "js_strings", "js_string_lines", "js_timing", "js_timeout", "js_timeout_sec"} {
		if _, exists := fields[old]; exists {
			t.Errorf("legacy key persisted: %s", old)
		}
	}
}

func TestRetiredAppearanceAndIdleSettingsPreserveOtherPreferences(t *testing.T) {
	for _, idle := range []string{"0", "30", "10080", "-1"} {
		var got Settings
		raw := []byte(`{"theme":"vague","daemon_idle_exit_min":` + idle + `,"notify":"done","view":"full"}`)
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.Notify != "done" || got.View != "full" {
			t.Fatalf("unrelated preferences changed: %+v", got)
		}
		encoded, _ := json.Marshal(got)
		var fields map[string]any
		_ = json.Unmarshal(encoded, &fields)
		for _, key := range []string{"theme", "daemon_idle_exit_min"} {
			if _, ok := fields[key]; ok {
				t.Fatalf("retired preference persisted: %s", key)
			}
		}
	}
}
