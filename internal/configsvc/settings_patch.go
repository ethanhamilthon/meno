package configsvc

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"meno/internal/api"
	"meno/internal/config"
)

// patchSettings applies (the caller holds settingsMu) a field-scoped change. Disjoint edits from different
// clients both survive because each only names the fields it changed; an edit
// of a field that moved since the client read it is a conflict.
func (s *Service) patchSettings(req api.ResourceRequest) (any, error) {
	patch, err := decode[api.SettingsPatch](req.Data)
	if err != nil {
		return nil, bad(err)
	}
	if len(patch.Fields) == 0 {
		return nil, badString("no settings fields to change")
	}
	cur, err := s.readSettingsLocked()
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(cur)
	var doc map[string]json.RawMessage
	if err = json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	var clash []string
	for name, want := range patch.Expect {
		if have, ok := doc[name]; ok && !sameJSON(have, want) {
			clash = append(clash, name)
		}
	}
	if len(clash) > 0 {
		sort.Strings(clash)
		return nil, conflict("settings changed elsewhere: " + strings.Join(clash, ", "))
	}
	for name, v := range patch.Fields {
		if _, ok := doc[name]; !ok {
			return nil, badString("unknown settings field: " + name)
		}
		doc[name] = v
	}
	merged, _ := json.Marshal(doc)
	var next config.Settings
	if err = json.Unmarshal(merged, &next); err != nil {
		return nil, bad(err)
	}
	if err = config.SaveSettingsFile(s.paths.SettingsFile(), next); err != nil {
		return nil, err
	}
	s.settings, s.settingsLoaded = next, true
	s.updateStamp(s.paths.SettingsFile(), s.stamp(s.paths.SettingsFile()))
	s.changed("settings_changed")
	return api.SettingsResponse{Settings: settingsWire(next)}, nil
}

func sameJSON(a, b json.RawMessage) bool {
	var x, y bytes.Buffer
	if json.Compact(&x, a) != nil || json.Compact(&y, b) != nil {
		return false
	}
	return bytes.Equal(x.Bytes(), y.Bytes())
}
