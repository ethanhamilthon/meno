package config

import (
	"encoding/json"
	"fmt"
	"os"

	"meno/internal/store"
)

// SettingsFileName is the daemon-readable settings file name.
const SettingsFileName = "settings.json"

// SettingsFile returns the settings path inside the meno home directory.
func (p Paths) SettingsFile() string { return p.File(SettingsFileName) }

// SaveSettingsFile writes the daemon-owned settings document atomically.
func SaveSettingsFile(path string, settings Settings) error { return store.SaveJSON(path, settings) }

// LoadCurrentSettings is the one place that decides where settings come from:
// settings.json when it exists, the legacy SQLite table only when the file is
// absent, and the defaults when neither holds saved values. A file that exists
// but cannot be read or parsed is an error: falling back to SQLite would select
// a different provider or model without telling the user.
func (s *Store) LoadCurrentSettings() (Settings, error) {
	b, err := os.ReadFile(s.P.SettingsFile())
	if err == nil {
		settings := DefaultSettings()
		if err := json.Unmarshal(b, &settings); err != nil {
			return DefaultSettings(), fmt.Errorf("settings file %s: %w", s.P.SettingsFile(), err)
		}
		return settings, nil
	}
	if !os.IsNotExist(err) {
		return DefaultSettings(), fmt.Errorf("settings file %s: %w", s.P.SettingsFile(), err)
	}
	return s.LoadSettings()
}
