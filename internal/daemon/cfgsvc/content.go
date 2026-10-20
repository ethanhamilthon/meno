package cfgsvc

import (
	"encoding/json"
	"net/url"
	"strings"

	"meno/internal/config"
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

func (s *Service) snippetsResource(method, path string, req proto.ResourceRequest) (any, error) {
	parts := splitResource(strings.TrimPrefix(path, "/snippets"))
	if len(parts) == 0 {
		switch method {
		case "GET":
			list, err := s.store.LoadSnippets()
			if err != nil {
				return nil, err
			}
			out := make([]proto.ConfigSnippet, 0, len(list))
			for _, x := range list {
				out = append(out, snippetWire(x))
			}
			return proto.SnippetsResponse{Snippets: out}, nil
		case "POST":
			r, err := decode[proto.SnippetRequest](req.Data)
			if err != nil {
				return nil, bad(err)
			}
			if err = s.store.SaveSnippet(r.Name, r.Text); err != nil {
				return nil, bad(err)
			}
			sn, err := s.store.Snippet(r.Name)
			if err != nil {
				return nil, err
			}
			s.changed("settings_changed")
			return proto.SnippetResponse{Snippet: snippetWire(sn)}, nil
		}
		return nil, notFound("unsupported snippets route")
	}
	name, err := url.PathUnescape(parts[0])
	if err != nil {
		return nil, bad(err)
	}
	if len(parts) == 1 && method == "DELETE" {
		if err := s.store.DeleteSnippet(name); err != nil {
			return nil, err
		}
		s.changed("settings_changed")
		return proto.AckResponse{OK: true}, nil
	}
	if len(parts) == 1 && method == "GET" {
		sn, err := s.store.Snippet(name)
		if err != nil {
			return nil, notFound(err.Error())
		}
		return proto.SnippetResponse{Snippet: snippetWire(sn)}, nil
	}
	if len(parts) == 2 && parts[1] == "rename" && method == "PUT" {
		r, err := decode[proto.SnippetRequest](req.Data)
		if err != nil {
			return nil, bad(err)
		}
		if err = s.store.RenameSnippet(name, r.Name); err != nil {
			return nil, bad(err)
		}
		sn, err := s.store.Snippet(r.Name)
		if err != nil {
			return nil, err
		}
		s.changed("settings_changed")
		return proto.SnippetResponse{Snippet: snippetWire(sn)}, nil
	}
	if len(parts) == 1 && method == "PUT" {
		r, err := decode[proto.SnippetRequest](req.Data)
		if err != nil {
			return nil, bad(err)
		}
		if r.Name != name {
			return nil, badString("snippet name cannot change; use /rename")
		}
		if err = s.store.SaveSnippet(r.Name, r.Text); err != nil {
			return nil, bad(err)
		}
		sn, err := s.store.Snippet(name)
		if err != nil {
			return nil, err
		}
		s.changed("settings_changed")
		return proto.SnippetResponse{Snippet: snippetWire(sn)}, nil
	}
	return nil, notFound("unsupported snippets route")
}
func snippetWire(s config.Snippet) proto.ConfigSnippet {
	b, _ := json.Marshal(s)
	var out proto.ConfigSnippet
	_ = json.Unmarshal(b, &out)
	return out
}
