package app

import (
	"errors"
	"fmt"
	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/jsvm"
	"meno/internal/llm"
	"meno/internal/prompt"
	"strings"
	"time"
)

// Spawn validates the request, builds the child's runtime and starts it.
func (p parentAgents) Spawn(tc jsvm.TaskCall, spec map[string]any) (any, error) {
	task, _ := spec["task"].(string)
	if strings.TrimSpace(task) == "" {
		return nil, errors.New("agents.spawn: task is required: the agent sees nothing of this conversation")
	}
	cwd := tc.Cwd
	if d, ok := spec["cwd"].(string); ok && d != "" {
		cwd = d
		if !strings.HasPrefix(d, "/") {
			cwd = tc.Cwd + "/" + d
		}
	}
	if !dirExists(cwd) {
		return nil, fmt.Errorf("agents.spawn: cwd %q is not a directory", cwd)
	}
	prov, model, effort := "", "", ""
	var pr llm.Provider
	var settings config.Settings
	var pricer func(llm.Usage) (float64, bool)
	var src worldSrc
	var err error
	p.a.onUI(func() {
		rt := p.a.rts[p.sid]
		if rt == nil {
			err = errors.New("the calling session is gone")
			return
		}
		prov, model, effort = rt.Session.Provider, rt.Session.Model, rt.Session.Effort
		for _, k := range []string{"provider", "model", "effort"} {
			v, set := spec[k]
			if !set {
				continue
			}
			s, ok := v.(string)
			if !ok || s == "" {
				err = fmt.Errorf("agents.spawn: %s must be a non-empty string", k)
				return
			}
			switch k {
			case "provider":
				prov = s
			case "model":
				model = s
			case "effort":
				effort = s
			}
		}
		if effort != "" && !contains(config.Choices("effort"), effort) {
			err = fmt.Errorf("agents.spawn: effort %q is not one of %v", effort, config.Choices("effort"))
			return
		}
		found := false
		for _, cand := range p.a.st.Providers {
			if cand.Name == prov {
				pr, found = cand, true
			}
		}
		if !found || model == "" {
			err = fmt.Errorf("agents.spawn: provider %q is not configured or has no model; see session.models()", prov)
			return
		}
		settings = p.a.st.Settings
		pricer = p.a.pricer(pr, model)
		src = p.a.worldSrc()
		src.Paths.Cwd = cwd
		src.Model = modelSig{prov, model, effort}
	})
	if err != nil {
		return nil, err
	}
	sess := agent.NewSession(cwd, model)
	sess.Kind, sess.ParentID, sess.Provider, sess.Effort = agent.KindChild, p.sid, prov, effort
	sess.AttachDir = p.a.attachDir()
	w, _ := src.gather(tc.Ctx)
	w.Input.Subagent = true
	system := prompt.Build(w.Input)
	c := &childRun{sess: sess, price: pricer, parent: p.sid}
	c.rec = agent.ChildRecord{ID: sess.ID, Parent: p.sid, Owner: p.requestID(), Task: task, Status: agent.ChildStarting, Gen: 1, Created: time.Now().UTC()}
	if err := p.m.build(c, pr, model, effort, system, settings); err != nil {
		return nil, err
	}
	sess.Append(agent.Turn{Role: llm.RoleUser, Text: task, Time: time.Now().UTC()})
	p.m.mu.Lock()
	if p.m.closing {
		p.m.mu.Unlock()
		return nil, errors.New("meno is shutting down")
	}
	p.m.runs[sess.ID] = c
	p.m.mu.Unlock()
	id := sess.ID
	p.m.start(c)
	return map[string]any{"id": id}, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
