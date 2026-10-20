package cfgsvc

import (
	"encoding/json"
	"os"

	"meno/internal/config"
	"meno/internal/proto"
)

func (s *Service) readSettings() (config.Settings, error) {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	path := s.paths.SettingsFile()
	stamp := s.stamp(path)
	changed := s.updateStamp(path, stamp)
	if !changed && s.settingsLoaded {
		return s.settings, nil
	}
	if changed && s.settingsLoaded {
		s.changed("settings_changed")
	}
	var settings config.Settings
	if stamp.exists {
		b, err := os.ReadFile(path)
		if err != nil {
			return settings, err
		}
		if err := json.Unmarshal(b, &settings); err != nil {
			return settings, err
		}
	} else {
		var err error
		settings, err = s.store.LoadSettings()
		if err != nil {
			return settings, err
		}
	}
	s.settings, s.settingsLoaded = settings, true
	return settings, nil
}

func (s *Service) settingsResource(method string, req proto.ResourceRequest) (any, error) {
	if err := checkMethod(method, "GET", "PUT"); err != nil {
		return nil, err
	}
	if method == "GET" {
		v, err := s.readSettings()
		if err != nil {
			return nil, err
		}
		return proto.SettingsResponse{Settings: settingsWire(v)}, nil
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
