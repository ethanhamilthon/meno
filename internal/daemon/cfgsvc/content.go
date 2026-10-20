package cfgsvc

import (
	"net/url"
	"strings"

	"meno/internal/proto"
)

func (s *Service) skillsResource(method, path string, req proto.ResourceRequest) (any, error) {
	parts := splitResource(strings.TrimPrefix(path, "/skills"))
	if len(parts) == 0 {
		if method != "GET" {
			return nil, notFound("unsupported skills route")
		}
		list, err := s.store.LoadSkills()
		if err != nil {
			return nil, err
		}
		out := make([]proto.ConfigSkill, 0, len(list))
		for _, x := range list {
			out = append(out, proto.ConfigSkill{Name: x.Name, Scope: x.Scope, Desc: x.Desc, Path: x.Path, Enabled: x.Enabled})
		}
		return proto.SkillsResponse{Skills: out}, nil
	}
	if len(parts) != 2 || parts[1] != "enabled" || (method != "PUT" && method != "PATCH") {
		return nil, notFound("unsupported skills route")
	}
	name, err := url.PathUnescape(parts[0])
	if err != nil {
		return nil, bad(err)
	}
	r, err := decode[proto.SkillEnabledRequest](req.Data)
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
			return proto.SkillResponse{Skill: proto.ConfigSkill{Name: x.Name, Scope: x.Scope, Desc: x.Desc, Path: x.Path, Enabled: x.Enabled}}, nil
		}
	}
	return nil, notFound("no skill named " + name)
}
