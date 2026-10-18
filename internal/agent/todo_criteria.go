package agent

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CriterionPatch holds the optional fields of a criterion update.
type CriterionPatch struct {
	Text, Status, Evidence, AgreementRef *string
	Refs                                 []string
}

// AddCriterion adds an acceptance criterion (status not_checked).
func (t *Todo) AddCriterion(text string) (Criterion, error) {
	text, err := checkText("text", text)
	if err != nil {
		return Criterion{}, err
	}
	var c Criterion
	err = t.mutate(func(p *Plan) error {
		p.NextCrit++
		c = Criterion{ID: "c" + strconv.Itoa(p.NextCrit), Text: text, Status: CritNotChecked}
		p.Criteria = append(p.Criteria, c)
		return nil
	})
	return c, err
}

func validRef(ref string) bool {
	kind, id, ok := strings.Cut(ref, ":")
	if !ok || id == "" {
		return false
	}
	switch kind {
	case "msg", "result", "task", "child", "artifact", "file":
		return !strings.ContainsAny(id, "\n\r") && len(id) <= 300
	}
	return false
}

// UpdateCriterion changes a criterion. passed and failed need evidence;
// deferred needs a reason and a reference to the user's agreement. The
// application checks structure, not whether the claim is true.
func (t *Todo) UpdateCriterion(id string, patch CriterionPatch) (Criterion, error) {
	var out Criterion
	err := t.mutate(func(p *Plan) error {
		idx := -1
		for i := range p.Criteria {
			if p.Criteria[i].ID == id {
				idx = i
			}
		}
		if idx < 0 {
			return fmt.Errorf("no criterion %q in the active plan", id)
		}
		c := p.Criteria[idx]
		if patch.Text != nil {
			v, err := checkText("text", *patch.Text)
			if err != nil {
				return err
			}
			c.Text = v
		}
		if patch.Evidence != nil {
			if len([]rune(*patch.Evidence)) > maxEvidence {
				return fmt.Errorf("evidence is longer than %d characters", maxEvidence)
			}
			c.Evidence = strings.TrimSpace(*patch.Evidence)
		}
		if patch.AgreementRef != nil {
			c.AgreementRef = strings.TrimSpace(*patch.AgreementRef)
		}
		for _, r := range patch.Refs {
			if !validRef(r) {
				return fmt.Errorf("reference %q is not msg:<id>, result:<id>, task:<id>, child:<id>, artifact:<id> or file:<path>", r)
			}
		}
		if patch.Refs != nil {
			c.Refs = patch.Refs
		}
		if patch.Status != nil {
			switch *patch.Status {
			case CritNotChecked, CritPassed, CritFailed, CritDeferred:
			default:
				return fmt.Errorf("unknown criterion status %q (not_checked, passed, failed, deferred)", *patch.Status)
			}
			c.Status = *patch.Status
		}
		switch c.Status {
		case CritPassed, CritFailed:
			if c.Evidence == "" && len(c.Refs) == 0 {
				return fmt.Errorf("a %s criterion needs evidence: what was checked and what was observed", c.Status)
			}
		case CritDeferred:
			if c.Evidence == "" {
				return errors.New("a deferred criterion needs the reason in evidence")
			}
			if c.AgreementRef == "" || !validRef(c.AgreementRef) {
				return errors.New("a deferred criterion needs agreementRef pointing at the message where the user agreed (msg:<id>)")
			}
		}
		p.Criteria[idx] = c
		out = c
		return nil
	})
	return out, err
}

// RemoveCriterion removes a criterion with a reason, leaving a tombstone.
func (t *Todo) RemoveCriterion(id, reason string) (bool, error) {
	reason, err := checkText("reason", reason)
	if err != nil {
		return false, err
	}
	found := false
	err = t.mutate(func(p *Plan) error {
		for i, c := range p.Criteria {
			if c.ID == id {
				p.Removed = append(p.Removed, Tombstone{Kind: "criterion", ID: id, Text: c.Text, Status: c.Status, Reason: reason, Time: time.Now().UTC()})
				p.Criteria = append(p.Criteria[:i], p.Criteria[i+1:]...)
				found = true
			}
		}
		return nil
	})
	return found, err
}

// Unverified reports the criteria that stop a plan from being fully done.
func (p *Plan) Unverified() []Criterion {
	var out []Criterion
	for _, c := range p.Criteria {
		if c.Status == CritNotChecked || c.Status == CritFailed {
			out = append(out, c)
		}
	}
	return out
}
