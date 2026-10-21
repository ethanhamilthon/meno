package cfgsvc

import "meno/internal/proto"

// bundleResource batches the same typed reads as the individual resources.
// Each read retains its own locking; no cross-store snapshot is promised.
func (s *Service) bundleResource(method string) (proto.ConfigBundle, error) {
	if err := checkMethod(method, "GET"); err != nil {
		return proto.ConfigBundle{}, err
	}
	var out proto.ConfigBundle
	var err error
	if out.Settings, err = s.settingsResponse(); err != nil {
		return proto.ConfigBundle{}, err
	}
	if out.Providers, err = s.providersResponse(); err != nil {
		return proto.ConfigBundle{}, err
	}
	if out.MCP, err = s.mcpResponse(); err != nil {
		return proto.ConfigBundle{}, err
	}
	if out.Skills, err = s.skillsResponse(); err != nil {
		return proto.ConfigBundle{}, err
	}
	return out, nil
}

func (s *Service) settingsResponse() (proto.SettingsResponse, error) {
	v, err := s.readSettings()
	if err != nil {
		return proto.SettingsResponse{}, err
	}
	return proto.SettingsResponse{Settings: settingsWire(v)}, nil
}

func (s *Service) providersResponse() (proto.ProvidersResponse, error) {
	ps, err := s.providers()
	if err != nil {
		return proto.ProvidersResponse{}, err
	}
	out := make([]proto.ConfigProvider, 0, len(ps))
	for _, p := range ps {
		out = append(out, providerWire(p))
	}
	return proto.ProvidersResponse{Providers: out}, nil
}

func (s *Service) mcpResponse() (proto.McpServersResponse, error) {
	list, warnings, err := s.serverDefinitions()
	if err != nil {
		return proto.McpServersResponse{}, err
	}
	out := make([]proto.ConfigMCPServer, 0, len(list))
	for _, x := range list {
		out = append(out, s.mcpWire(x))
	}
	return proto.McpServersResponse{Servers: out, Warnings: warnings}, nil
}

func (s *Service) skillsResponse() (proto.SkillsResponse, error) {
	list, err := s.store.LoadSkills()
	if err != nil {
		return proto.SkillsResponse{}, err
	}
	out := make([]proto.ConfigSkill, 0, len(list))
	for _, x := range list {
		out = append(out, proto.ConfigSkill{Name: x.Name, Scope: x.Scope, Desc: x.Desc, Path: x.Path, Enabled: x.Enabled})
	}
	return proto.SkillsResponse{Skills: out}, nil
}
