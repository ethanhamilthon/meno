package app

import (
	"errors"
	"fmt"

	"meno/internal/agent"
)

// todoHost implements jsvm.TodoHost for one session and keeps the UI copy of
// the plan current (rendering never reads the database).
type todoHost struct {
	a    *App
	sid  string
	todo *agent.Todo
}

type todoChanged struct {
	sid  string
	plan *agent.Plan
}

func (h todoHost) changed() {
	p, _ := h.todo.Active()
	h.a.postEvent(todoChanged{h.sid, p})
}

func strOpt(o map[string]any, k string) (*string, error) {
	v, ok := o[k]
	if !ok {
		return nil, nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("%s must be a string", k)
	}
	return &s, nil
}

func onlyKeys(o map[string]any, allowed ...string) error {
outer:
	for k := range o {
		for _, a := range allowed {
			if a == k {
				continue outer
			}
		}
		return fmt.Errorf("unknown option %q (allowed: %v)", k, allowed)
	}
	return nil
}

func (h todoHost) List(includeArchived bool) (any, error) {
	p, err := h.todo.Active()
	if err != nil {
		return nil, err
	}
	out := map[string]any{"plan": nil, "items": []any{}, "criteria": []any{}, "removed": []any{}}
	if p != nil {
		out["plan"] = map[string]any{"id": p.ID, "created": p.Created}
		out["items"], out["criteria"], out["removed"] = p.Items, p.Criteria, p.Removed
		if un := p.Unverified(); len(un) > 0 {
			out["unverified"] = len(un)
		}
	}
	if includeArchived {
		arch, err := h.todo.Archived()
		if err != nil {
			return nil, err
		}
		out["archived"] = arch
	}
	return toAny(out)
}

func (h todoHost) Add(o map[string]any) (any, error) {
	if err := onlyKeys(o, "title", "note"); err != nil {
		return nil, err
	}
	title, err := strOpt(o, "title")
	if err != nil || title == nil {
		return nil, errors.Join(errors.New("title is required"), err)
	}
	note, err := strOpt(o, "note")
	if err != nil {
		return nil, err
	}
	n := ""
	if note != nil {
		n = *note
	}
	it, err := h.todo.AddItem(*title, n)
	if err != nil {
		return nil, err
	}
	h.changed()
	return toAny(it)
}

func (h todoHost) Update(id string, o map[string]any) (any, error) {
	if err := onlyKeys(o, "title", "note", "status", "position"); err != nil {
		return nil, err
	}
	var patch agent.ItemPatch
	var err error
	if patch.Title, err = strOpt(o, "title"); err != nil {
		return nil, err
	}
	if patch.Note, err = strOpt(o, "note"); err != nil {
		return nil, err
	}
	if patch.Status, err = strOpt(o, "status"); err != nil {
		return nil, err
	}
	if v, ok := o["position"]; ok {
		n, ok := v.(int64)
		if !ok {
			return nil, errors.New("position must be an integer")
		}
		pos := int(n)
		patch.Position = &pos
	}
	it, err := h.todo.UpdateItem(id, patch)
	if err != nil {
		return nil, err
	}
	h.changed()
	return toAny(it)
}

func (h todoHost) Done(id string, o map[string]any) (any, error) {
	if err := onlyKeys(o, "note"); err != nil {
		return nil, err
	}
	note, err := strOpt(o, "note")
	if err != nil {
		return nil, err
	}
	n := ""
	if note != nil {
		n = *note
	}
	it, err := h.todo.DoneItem(id, n)
	if err != nil {
		return nil, err
	}
	h.changed()
	return toAny(it)
}

func (h todoHost) Remove(id string) (bool, error) {
	ok, err := h.todo.RemoveItem(id)
	if err == nil {
		h.changed()
	}
	return ok, err
}

func (h todoHost) Archive(o map[string]any) (any, error) {
	if err := onlyKeys(o, "reason"); err != nil {
		return nil, err
	}
	reason, err := strOpt(o, "reason")
	if err != nil || reason == nil {
		return nil, errors.Join(errors.New("reason is required"), err)
	}
	p, err := h.todo.Archive(*reason)
	if err != nil {
		return nil, err
	}
	h.changed()
	return toAny(p)
}

func (h todoHost) CriteriaList() (any, error) {
	p, err := h.todo.Active()
	if err != nil || p == nil {
		return []any{}, err
	}
	return toAny(p.Criteria)
}

func (h todoHost) CriteriaAdd(o map[string]any) (any, error) {
	if err := onlyKeys(o, "text"); err != nil {
		return nil, err
	}
	text, err := strOpt(o, "text")
	if err != nil || text == nil {
		return nil, errors.Join(errors.New("text is required"), err)
	}
	c, err := h.todo.AddCriterion(*text)
	if err != nil {
		return nil, err
	}
	h.changed()
	return toAny(c)
}

func (h todoHost) CriteriaUpdate(id string, o map[string]any) (any, error) {
	if err := onlyKeys(o, "text", "status", "evidence", "agreementRef", "refs"); err != nil {
		return nil, err
	}
	var p agent.CriterionPatch
	var err error
	if p.Text, err = strOpt(o, "text"); err != nil {
		return nil, err
	}
	if p.Status, err = strOpt(o, "status"); err != nil {
		return nil, err
	}
	if p.Evidence, err = strOpt(o, "evidence"); err != nil {
		return nil, err
	}
	if p.AgreementRef, err = strOpt(o, "agreementRef"); err != nil {
		return nil, err
	}
	if v, ok := o["refs"]; ok {
		list, ok := v.([]any)
		if !ok {
			return nil, errors.New("refs must be an array of strings")
		}
		p.Refs = []string{}
		for _, r := range list {
			s, ok := r.(string)
			if !ok {
				return nil, errors.New("refs must be an array of strings")
			}
			p.Refs = append(p.Refs, s)
		}
	}
	c, err := h.todo.UpdateCriterion(id, p)
	if err != nil {
		return nil, err
	}
	h.changed()
	return toAny(c)
}

func (h todoHost) CriteriaRemove(id string, o map[string]any) (bool, error) {
	if err := onlyKeys(o, "reason"); err != nil {
		return false, err
	}
	reason, err := strOpt(o, "reason")
	if err != nil || reason == nil {
		return false, errors.Join(errors.New("reason is required"), err)
	}
	ok, err := h.todo.RemoveCriterion(id, *reason)
	if err == nil {
		h.changed()
	}
	return ok, err
}
