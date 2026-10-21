package cfgsvc

import (
	"encoding/json"

	"meno/internal/config"
	"meno/internal/proto"
)

func (s *Service) readSettings() (config.Settings, error) {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	return s.readSettingsLocked()
}

// readSettingsLocked is readSettings for a caller that holds settingsMu.
func (s *Service) readSettingsLocked() (config.Settings, error) {
	path := s.paths.SettingsFile()
	stamp := s.stamp(path)
	changed := s.updateStamp(path, stamp)
	if !changed && s.settingsLoaded {
		return s.settings, nil
	}
	if changed && s.settingsLoaded {
		s.changed("settings_changed")
	}
	settings, err := s.store.LoadCurrentSettings()
	if err != nil {
		return settings, err
	}
	s.settings, s.settingsLoaded = settings, true
	return settings, nil
}

// Settings is the current settings snapshot for the engine inside the daemon
// process. It is the same snapshot the settings route serves.
func (s *Service) Settings() (config.Settings, error) { return s.readSettings() }

func (s *Service) settingsResource(method string, req proto.ResourceRequest) (any, error) {
	if err := checkMethod(method, "GET", "PUT", "PATCH"); err != nil {
		return nil, err
	}
	if method == "PATCH" {
		s.settingsMu.Lock()
		defer s.settingsMu.Unlock()
		return s.patchSettings(req)
	}
	if method == "GET" {
		return s.settingsResponse()
	}
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	var wire proto.SettingsRequest
	if err := json.Unmarshal(req.Data, &wire); err != nil {
		return nil, bad(err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(req.Data, &envelope); err != nil {
		return nil, bad(err)
	}
	if len(envelope["settings"]) == 0 {
		return nil, badString("settings object is required")
	}

	b, err := json.Marshal(wire.Settings)
	if err != nil {
		return nil, err
	}
	var next config.Settings
	if err := json.Unmarshal(b, &next); err != nil {
		return nil, bad(err)
	}

	if err := config.SaveSettingsFile(s.paths.SettingsFile(), next); err != nil {
		return nil, err
	}
	s.settings, s.settingsLoaded = next, true
	s.updateStamp(s.paths.SettingsFile(), s.stamp(s.paths.SettingsFile()))
	s.changed("settings_changed")
	return proto.SettingsResponse{Settings: settingsWire(next)}, nil
}

func settingsWire(s config.Settings) proto.ConfigSettings {
	b, _ := json.Marshal(s)
	var out proto.ConfigSettings
	_ = json.Unmarshal(b, &out)
	return out
}
