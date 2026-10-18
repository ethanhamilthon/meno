package agent

import (
	"strings"
	"testing"

	"meno/internal/llm"
)

func newTodo(t *testing.T, s *Session) (*Todo, string) {
	t.Helper()
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	td, err := NewTodo(home, s.ID, s.ExecBoundary)
	if err != nil {
		t.Fatal(err)
	}
	return td, home
}

func TestTodoStepsCriteriaAndRules(t *testing.T) {
	s := NewSession("/w", "m")
	td, _ := newTodo(t, s)
	a, err := td.AddItem("write code", "")
	if err != nil || a.ID != "i1" || a.Status != ItemPending {
		t.Fatalf("%+v %v", a, err)
	}
	b, _ := td.AddItem("write tests", "after code")
	pos := 0
	if _, err := td.UpdateItem(b.ID, ItemPatch{Position: &pos}); err != nil {
		t.Fatal(err)
	}
	p, _ := td.Active()
	if p.Items[0].ID != b.ID || p.Items[1].ID != a.ID {
		t.Fatalf("position: %+v", p.Items)
	}
	bad := "weird"
	if _, err := td.UpdateItem(a.ID, ItemPatch{Status: &bad}); err == nil {
		t.Fatal("unknown status")
	}
	if it, err := td.DoneItem(a.ID, "done in 3 files"); err != nil || it.Status != ItemDone || it.Note != "done in 3 files" {
		t.Fatalf("%+v %v", it, err)
	}
	c, _ := td.AddCriterion("all tests pass")
	// A done step does not pass a criterion; passed/failed/deferred need support.
	passed := CritPassed
	if _, err := td.UpdateCriterion(c.ID, CriterionPatch{Status: &passed}); err == nil || !strings.Contains(err.Error(), "evidence") {
		t.Fatalf("passed without evidence: %v", err)
	}
	ev := "go test ./... : ok, 14 packages"
	if got, err := td.UpdateCriterion(c.ID, CriterionPatch{Status: &passed, Evidence: &ev, Refs: []string{"result:r1"}}); err != nil || got.Status != CritPassed {
		t.Fatalf("%+v %v", got, err)
	}
	deferred := CritDeferred
	reason := "needs hardware"
	if _, err := td.UpdateCriterion(c.ID, CriterionPatch{Status: &deferred, Evidence: &reason}); err == nil || !strings.Contains(err.Error(), "agreementRef") {
		t.Fatalf("deferred needs the user's agreement: %v", err)
	}
	bogus := "the user said ok"
	if _, err := td.UpdateCriterion(c.ID, CriterionPatch{Status: &deferred, Evidence: &reason, AgreementRef: &bogus}); err == nil {
		t.Fatal("agreementRef must be a structured reference")
	}
	agree := "msg:12"
	if _, err := td.UpdateCriterion(c.ID, CriterionPatch{Status: &deferred, Evidence: &reason, AgreementRef: &agree}); err != nil {
		t.Fatal(err)
	}
	// Removal leaves a tombstone and unverified criteria stay visible.
	c2, _ := td.AddCriterion("works on linux")
	if _, err := td.RemoveCriterion(c2.ID, ""); err == nil {
		t.Fatal("removal needs a reason")
	}
	if ok, err := td.RemoveCriterion(c2.ID, "out of scope by agreement"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	p, _ = td.Active()
	if len(p.Removed) != 1 || p.Removed[0].Status != CritNotChecked || len(p.Criteria) != 1 {
		t.Fatalf("%+v", p)
	}
	// Archive, then adding starts a new plan with a new ID.
	old := p.ID
	if _, err := td.Archive("replanned"); err != nil {
		t.Fatal(err)
	}
	if cur, _ := td.Active(); cur != nil {
		t.Fatal("no active plan after archive")
	}
	td.AddItem("fresh start", "")
	cur, _ := td.Active()
	arch, _ := td.Archived()
	if cur.ID == old || len(arch) != 1 || arch[0].ID != old || arch[0].Reason != "replanned" || len(cur.Items) != 1 {
		t.Fatalf("cur %+v archived %+v", cur, arch)
	}
	if _, err := td.Archive(""); err == nil {
		t.Fatal("archive needs a reason")
	}
}

// A fork sees the plan as it was at its boundary, and a child's plan is its own.
func TestTodoVersionsFollowForksAndSessionsAreIsolated(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession(t.TempDir(), "m")
	td, _ := NewTodo(home, s.ID, s.ExecBoundary)
	s.Append(Turn{Role: llm.RoleUser, Text: "go"}) // 1
	td.AddItem("first", "")
	s.Append(Turn{Role: llm.RoleAssistant, Text: "a", CallID: "c", Code: "x"}) // 2
	td.AddItem("second", "")
	s.Append(Turn{Role: llm.RoleAssistant, Text: "b", CallID: "d", Code: "y"}) // 3
	if err := SaveSession(home, s); err != nil {
		t.Fatal(err)
	}
	f, _, err := ForkSession(home, s.ID, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	ftd, _ := NewTodo(home, f.ID, f.ExecBoundary)
	fp, _ := ftd.Active()
	if fp == nil || len(fp.Items) != 1 || fp.Items[0].Title != "first" {
		t.Fatalf("fork plan: %+v", fp)
	}
	ftd.AddItem("fork only", "")
	if sp, _ := td.Active(); len(sp.Items) != 2 {
		t.Fatalf("source plan changed by the fork: %+v", sp)
	}
	other, _ := NewTodo(home, "child-1", func() int64 { return 1 })
	if p, _ := other.Active(); p != nil {
		t.Fatal("another session must not see this plan")
	}
}
