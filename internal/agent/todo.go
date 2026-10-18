package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Item and criterion statuses.
const (
	ItemPending    = "pending"
	ItemInProgress = "in_progress"
	ItemBlocked    = "blocked"
	ItemDone       = "done"
	ItemCancelled  = "cancelled"

	CritNotChecked = "not_checked"
	CritPassed     = "passed"
	CritFailed     = "failed"
	CritDeferred   = "deferred"
)

const (
	maxTodoText  = 500
	maxEvidence  = 2000
	maxPlanItems = 200
)

// Item is a step of the plan. IDs are stable and never reused.
type Item struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Note   string `json:"note,omitempty"`
	Status string `json:"status"`
}

// Criterion is an acceptance criterion, independent of the steps.
type Criterion struct {
	ID           string   `json:"id"`
	Text         string   `json:"text"`
	Status       string   `json:"status"`
	Evidence     string   `json:"evidence,omitempty"`
	Refs         []string `json:"refs,omitempty"`
	AgreementRef string   `json:"agreementRef,omitempty"`
}

// Tombstone records a removal; removed work stays auditable.
type Tombstone struct {
	Kind   string    `json:"kind"`
	ID     string    `json:"id"`
	Text   string    `json:"text"`
	Status string    `json:"status"`
	Reason string    `json:"reason,omitempty"`
	Time   time.Time `json:"time"`
}

// Plan is the one active plan of a session.
type Plan struct {
	ID         string      `json:"id"`
	Created    time.Time   `json:"created"`
	Items      []Item      `json:"items"`
	Criteria   []Criterion `json:"criteria"`
	Removed    []Tombstone `json:"removed,omitempty"`
	NextItem   int         `json:"nextItem"`
	NextCrit   int         `json:"nextCriterion"`
	Archived   bool        `json:"archived,omitempty"`
	ArchivedAt *time.Time  `json:"archivedAt,omitempty"`
	Reason     string      `json:"archiveReason,omitempty"`
}

// Todo manages the plan of one session on top of its versioned store, so every
// mutation is a journal entry and a fork sees the plan as it was.
type Todo struct {
	kv       *KV
	Boundary func() int64 // the history boundary of the step that is executing
}

const planKey = "plan"

// NewTodo returns the plan manager of a session.
func NewTodo(dir, session string, boundary func() int64) (*Todo, error) {
	kv, err := OpenKV(dir, session, NSTodo)
	if err != nil {
		return nil, err
	}
	return &Todo{kv: kv, Boundary: boundary}, nil
}

// Active returns the active plan, or nil when there is none.
func (t *Todo) Active() (*Plan, error) {
	raw, ok, err := t.kv.Get(planKey)
	if err != nil || !ok {
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, fmt.Errorf("stored plan is damaged: %w", err)
	}
	return &p, nil
}

// Archived lists the archived plans, oldest first.
func (t *Todo) Archived() ([]Plan, error) {
	es, err := t.kv.Entries()
	if err != nil {
		return nil, err
	}
	var out []Plan
	for _, e := range es {
		if strings.HasPrefix(e.Key, "archive/") {
			var p Plan
			if json.Unmarshal([]byte(e.JSON), &p) == nil {
				out = append(out, p)
			}
		}
	}
	return out, nil
}

func (t *Todo) save(p *Plan) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return t.kv.Set(planKey, string(b), t.Boundary())
}

// mutate loads (or starts) the plan, applies fn and stores the result. A
// failing fn changes nothing.
func (t *Todo) mutate(fn func(p *Plan) error) error {
	p, err := t.Active()
	if err != nil {
		return err
	}
	if p == nil {
		p = &Plan{ID: "p" + NewID()[16:22], Created: time.Now().UTC()}
	}
	if err := fn(p); err != nil {
		return err
	}
	return t.save(p)
}

func checkText(what, s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("%s must not be empty", what)
	}
	if len([]rune(s)) > maxTodoText {
		return "", fmt.Errorf("%s is longer than %d characters", what, maxTodoText)
	}
	return s, nil
}

func validItemStatus(s string) bool {
	switch s {
	case ItemPending, ItemInProgress, ItemBlocked, ItemDone, ItemCancelled:
		return true
	}
	return false
}

// ItemPatch holds the optional fields of an item update.
type ItemPatch struct {
	Title, Note, Status *string
	Position            *int
}

// AddItem appends a step. Adding after an archive starts a new plan.
func (t *Todo) AddItem(title, note string) (Item, error) {
	title, err := checkText("title", title)
	if err != nil {
		return Item{}, err
	}
	if len([]rune(note)) > maxEvidence {
		return Item{}, fmt.Errorf("note is longer than %d characters", maxEvidence)
	}
	var it Item
	err = t.mutate(func(p *Plan) error {
		if len(p.Items) >= maxPlanItems {
			return fmt.Errorf("a plan has at most %d steps; archive it or remove finished ones", maxPlanItems)
		}
		p.NextItem++
		it = Item{ID: "i" + strconv.Itoa(p.NextItem), Title: title, Note: strings.TrimSpace(note), Status: ItemPending}
		p.Items = append(p.Items, it)
		return nil
	})
	return it, err
}

// UpdateItem changes fields of a step.
func (t *Todo) UpdateItem(id string, patch ItemPatch) (Item, error) {
	var out Item
	err := t.mutate(func(p *Plan) error {
		idx := -1
		for i := range p.Items {
			if p.Items[i].ID == id {
				idx = i
			}
		}
		if idx < 0 {
			return fmt.Errorf("no step %q in the active plan", id)
		}
		it := p.Items[idx]
		if patch.Title != nil {
			v, err := checkText("title", *patch.Title)
			if err != nil {
				return err
			}
			it.Title = v
		}
		if patch.Note != nil {
			if len([]rune(*patch.Note)) > maxEvidence {
				return fmt.Errorf("note is longer than %d characters", maxEvidence)
			}
			it.Note = strings.TrimSpace(*patch.Note)
		}
		if patch.Status != nil {
			if !validItemStatus(*patch.Status) {
				return fmt.Errorf("unknown step status %q (pending, in_progress, blocked, done, cancelled)", *patch.Status)
			}
			it.Status = *patch.Status
		}
		p.Items[idx] = it
		if patch.Position != nil {
			pos := *patch.Position
			if pos < 0 || pos >= len(p.Items) {
				return fmt.Errorf("position %d is outside 0..%d", pos, len(p.Items)-1)
			}
			p.Items = append(p.Items[:idx], p.Items[idx+1:]...)
			p.Items = append(p.Items[:pos], append([]Item{it}, p.Items[pos:]...)...)
		}
		out = it
		return nil
	})
	return out, err
}

// DoneItem marks a step done. It does not pass any criterion.
func (t *Todo) DoneItem(id, note string) (Item, error) {
	st := ItemDone
	patch := ItemPatch{Status: &st}
	if note != "" {
		patch.Note = &note
	}
	return t.UpdateItem(id, patch)
}

// RemoveItem removes a step and leaves a tombstone.
func (t *Todo) RemoveItem(id string) (bool, error) {
	found := false
	err := t.mutate(func(p *Plan) error {
		for i, it := range p.Items {
			if it.ID == id {
				p.Removed = append(p.Removed, Tombstone{Kind: "step", ID: id, Text: it.Title, Status: it.Status, Time: time.Now().UTC()})
				p.Items = append(p.Items[:i], p.Items[i+1:]...)
				found = true
				return nil
			}
		}
		return nil
	})
	if err == nil && !found {
		if p, _ := t.Active(); p == nil {
			return false, nil
		}
	}
	return found, err
}

// Archive closes the active plan with a reason; the next add starts a new one.
func (t *Todo) Archive(reason string) (Plan, error) {
	reason, err := checkText("reason", reason)
	if err != nil {
		return Plan{}, err
	}
	p, err := t.Active()
	if err != nil {
		return Plan{}, err
	}
	if p == nil {
		return Plan{}, errors.New("there is no active plan to archive")
	}
	now := time.Now().UTC()
	p.Archived, p.ArchivedAt, p.Reason = true, &now, reason
	b, err := json.Marshal(p)
	if err != nil {
		return Plan{}, err
	}
	if err := t.kv.Set("archive/"+p.ID, string(b), t.Boundary()); err != nil {
		return Plan{}, err
	}
	if _, err := t.kv.Delete(planKey, t.Boundary()); err != nil {
		return Plan{}, err
	}
	return *p, nil
}
