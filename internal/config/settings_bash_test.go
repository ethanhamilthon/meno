package config

import (
	"encoding/json"
	"testing"
)

func TestBashDefaultsAliasesAndSettingWrite(t *testing.T) {
	d := DefaultSettings()
	if d.Tool != "bash" || d.BashTimeout != "120" || d.DaemonIdleExitMin != 30 {
		t.Fatalf("defaults: %+v", d)
	}
	var legacy Settings
	if err := json.Unmarshal([]byte("{\"js_code\":\"lines\",\"js_lines\":8,\"js_output\":\"off\",\"js_output_lines\":9,\"js_strings\":\"lines\",\"js_string_lines\":4,\"js_timing\":\"slow\",\"js_timeout\":\"sec\",\"js_timeout_sec\":45}"), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.BashCode != "lines" || legacy.BashLines != 8 || legacy.BashOutput != "off" || legacy.BashOutputLines != 9 || legacy.BashStrings != "lines" || legacy.BashStringLines != 4 || legacy.BashTiming != "slow" || legacy.BashTimeout != "45" || legacy.Tool != "js" {
		t.Fatalf("alias: %+v", legacy)
	}
	for key, value := range map[string]string{"tool": "bash", "bash_timeout": "off", "bash_code": "lines", "bash_lines": "8", "daemon_idle_exit_min": "0"} {
		if err := d.Set(key, value); err != nil {
			t.Errorf("%s: %v", key, err)
		}
	}
	if d.Tool != "bash" || d.BashTimeout != "off" || d.BashLines != 8 || d.DaemonIdleExitMin != 0 {
		t.Fatalf("set: %+v", d)
	}
	if got := d.Cycle("bash_timeout", 1); got != "model" || d.BashTimeout != "model" {
		t.Fatalf("timeout cycle to model: %q, %q", got, d.BashTimeout)
	}
	if got := d.Cycle("bash_timeout", 1); got != "seconds" || d.BashTimeout != "120" {
		t.Fatalf("timeout cycle to seconds: %q, %q", got, d.BashTimeout)
	}
	if got := d.Cycle("bash_timeout", 1); got != "off" || d.BashTimeout != "off" {
		t.Fatalf("timeout cycle to off: %q, %q", got, d.BashTimeout)
	}
	if d.Cycle("tool", -1) != "js" {
		t.Fatal("tool cycle")
	}
	if err := d.Set("bash_timeout", "model"); err != nil || !d.BashModelTimeout() {
		t.Fatalf("model: %v", err)
	}
	for key, value := range map[string]string{"bash_timeout": "0", "bash_lines": "0", "daemon_idle_exit_min": "-1"} {
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
