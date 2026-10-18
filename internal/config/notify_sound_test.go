package config

import (
	"encoding/json"
	"testing"
)

func TestNotifySoundDefaultsAndRoundTrip(t *testing.T) {
	for _, data := range []string{
		"{}", `{"notify_sound":"unknown"}`, `{"notify_sound":""}`,
	} {
		var s Settings
		if err := json.Unmarshal([]byte(data), &s); err != nil || s.NotifySound != "default" {
			t.Fatalf("%s: %+v %v", data, s, err)
		}
	}
	for _, name := range Choices("notify_sound") {
		s := DefaultSettings()
		if err := s.Set("notify_sound", name); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var got Settings
		if err := json.Unmarshal(data, &got); err != nil || got != s {
			t.Fatalf("%s: %+v %v", name, got, err)
		}
	}
}
