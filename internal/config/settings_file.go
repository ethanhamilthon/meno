package config

import "meno/internal/store"

// SettingsFileName is the daemon-readable settings file name.
const SettingsFileName = "settings.json"

// SettingsFile returns the settings path inside the meno home directory.
func (p Paths) SettingsFile() string { return p.File(SettingsFileName) }

// SaveSettingsFile writes the daemon-owned settings document atomically.
func SaveSettingsFile(path string, settings Settings) error { return store.SaveJSON(path, settings) }
