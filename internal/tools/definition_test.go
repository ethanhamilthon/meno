package tools

import (
	"encoding/json"
	"testing"
)

func TestParseTimeoutSetting(t *testing.T) {
	tests := []struct {
		value   string
		mode    TimeoutMode
		seconds int
		invalid bool
	}{
		{"", TimeoutSeconds, 120, false}, {"120", TimeoutSeconds, 120, false}, {"5", TimeoutSeconds, 5, false},
		{"off", TimeoutOff, 0, false}, {"model", TimeoutModel, 0, false}, {"0", "", 0, true}, {"-2", "", 0, true}, {"many", "", 0, true},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := ParseTimeoutSetting(test.value)
			if (err != nil) != test.invalid {
				t.Fatalf("ParseTimeoutSetting() = %#v, %v", got, err)
			}
			if err == nil && (got.Mode != test.mode || got.Seconds != test.seconds) {
				t.Fatalf("got %#v", got)
			}
		})
	}
}

func TestBashToolDefinitionTimeoutIsModelOnly(t *testing.T) {
	for _, test := range []struct {
		mode       TimeoutMode
		hasTimeout bool
	}{{TimeoutSeconds, false}, {TimeoutOff, false}, {TimeoutModel, true}} {
		definition := BashToolDefinition(TimeoutSetting{Mode: test.mode})
		if definition.Name != "bash" {
			t.Fatalf("name = %q", definition.Name)
		}
		properties := definition.Parameters["properties"].(map[string]any)
		_, hasTimeout := properties["timeout"]
		if hasTimeout != test.hasTimeout {
			t.Errorf("timeout property = %v for %q", hasTimeout, test.mode)
		}
		encoded, err := json.Marshal(definition)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
	}
}
