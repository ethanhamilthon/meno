package configsvc

import (
	"net/url"
	"strings"

	"meno/internal/api"
)

func (s *Service) skillsResource(method, path string, req api.ResourceRequest) (any, error) {
	parts := splitResource(strings.TrimPrefix(path, "/skills"))
	if len(parts) == 0 {
		if method != "GET" {
			return nil, notFound("unsupported skills route")
		}
		return s.skillsResponse()
	}
	if len(parts) != 2 || parts[1] != "enabled" || (method != "PUT" && method != "PATCH") {
		return nil, notFound("unsupported skills route")
	}
	name, err := url.PathUnescape(parts[0])
	if err != nil {
		return nil, bad(err)
	}
	r, err := decode[api.SkillEnabledRequest](req.Data)
	if err != nil {
		return nil, bad(err)
	}
	list, err := s.store.LoadSkills()
	if err != nil {
		return nil, err
	}
	found := false
	for i := range list {
		if list[i].Name == name {
			list[i].Enabled = r.Enabled
			found = true
		}
	}
	if !found {
		return nil, notFound("no skill named " + name)
	}
	if err := s.store.SaveSkills(list); err != nil {
		return nil, err
	}
	s.changed("settings_changed")
	for _, x := range list {
		if x.Name == name {
			return api.SkillResponse{Skill: api.ConfigSkill{Name: x.Name, Scope: x.Scope, Desc: x.Desc, Path: x.Path, Enabled: x.Enabled, Warnings: x.Warnings}}, nil
		}
	}
	return nil, notFound("no skill named " + name)
}
