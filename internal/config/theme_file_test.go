package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSingleThemeInitializationAndLegacyBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			if _, e := LoadTheme(path); e != nil {
				t.Error(e)
			}
		})
	}
	wg.Wait()
	got, e := ReadTheme(path)
	if e != nil || got != DefaultThemeColors() {
		t.Fatalf("%+v %v", got, e)
	}
	colors := DefaultThemeColors()
	colors.Blue = "#123456"
	raw, _ := json.Marshal(colors)
	legacy := append([]byte(`[{"name":"other"},`), append(append([]byte(`{"name":"panel-dark",`), raw[1:]...), ']')...)
	if e = os.WriteFile(path, legacy, 0600); e != nil {
		t.Fatal(e)
	}
	got, e = LoadTheme(path)
	if e != nil || got != colors {
		t.Fatalf("migration %+v %v", got, e)
	}
	backup, _ := os.ReadFile(path + ".legacy.bak")
	if string(backup) != string(legacy) {
		t.Fatal("backup changed")
	}
	if _, e = LoadTheme(path); e != nil {
		t.Fatal(e)
	}
}

func TestThemeRejectsPartialOrInvalidWithoutOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	for _, raw := range []string{`{}`, `null`, `[]`, `{"bg":"bad"}`, `{"dark":7}`, `{`} {
		if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := ReadTheme(path); e == nil {
			t.Errorf("accepted %s", raw)
		}
		if raw != "[]" {
			_, _ = LoadTheme(path)
			got, _ := os.ReadFile(path)
			if string(got) != raw {
				t.Fatal("invalid file overwritten")
			}
		}
	}
}
