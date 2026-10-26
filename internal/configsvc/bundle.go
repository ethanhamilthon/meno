package configsvc

import (
	"meno/internal/api"
	"meno/internal/config"
)

// bundleResource batches the same typed reads as the individual resources.
// Each read retains its own locking; no cross-store snapshot is promised.
func (s *Service) bundleResource(method string) (api.ConfigBundle, error) {
	if err := checkMethod(method, "GET"); err != nil {
		return api.ConfigBundle{}, err
	}
	var out api.ConfigBundle
	var err error
	if out.Settings, err = s.settingsResponse(); err != nil {
		return api.ConfigBundle{}, err
	}
	if out.Providers, err = s.providersResponse(); err != nil {
		return api.ConfigBundle{}, err
	}
	if out.MCP, err = s.mcpResponse(); err != nil {
		return api.ConfigBundle{}, err
	}
	if out.Skills, err = s.skillsResponse(""); err != nil {
		return api.ConfigBundle{}, err
	}
	return out, nil
}

func (s *Service) settingsResponse() (api.SettingsResponse, error) {
	v, err := s.readSettings()
	if err != nil {
		return api.SettingsResponse{}, err
	}
	return api.SettingsResponse{Settings: settingsWire(v)}, nil
}

func (s *Service) providersResponse() (api.ProvidersResponse, error) {
	ps, warnings, err := s.providerDefinitions()
	if err != nil {
		return api.ProvidersResponse{}, err
	}
	out := make([]api.ConfigProvider, 0, len(ps))
	for _, p := range ps {
		out = append(out, providerWire(p))
	}
	return api.ProvidersResponse{Providers: out, Warnings: warnings}, nil
}

func (s *Service) mcpResponse() (api.McpServersResponse, error) {
	list, warnings, err := s.serverDefinitions()
	if err != nil {
		return api.McpServersResponse{}, err
	}
	out := make([]api.ConfigMCPServer, 0, len(list))
	for _, x := range list {
		out = append(out, s.mcpWire(x))
	}
	return api.McpServersResponse{Servers: out, Warnings: warnings}, nil
}

// skillsResponse lists the skills seen from cwd: the project skills belong to
// the directory of the asking session, not to the one the daemon started in.
func (s *Service) skillsResponse(cwd string) (api.SkillsResponse, error) {
	list, err := s.storeAt(cwd).LoadSkills()
	if err != nil {
		return api.SkillsResponse{}, err
	}
	out := make([]api.ConfigSkill, 0, len(list))
	for _, x := range list {
		out = append(out, api.ConfigSkill{Name: x.Name, Scope: x.Scope, Desc: x.Desc, Path: x.Path, Enabled: x.Enabled, Auto: x.Auto, Warnings: x.Warnings})
	}
	return api.SkillsResponse{Skills: out}, nil
}

// storeAt is the store seen from cwd; "" is the directory of the daemon.
func (s *Service) storeAt(cwd string) *config.Store {
	if cwd == "" {
		return s.store
	}
	paths := s.paths
	paths.Cwd = cwd
	return config.NewStore(paths)
}
