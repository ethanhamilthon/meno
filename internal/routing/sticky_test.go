package routing

import "testing"

func TestStickyRequiresTwoMatchingVerdicts(t *testing.T) {
	s := Sticky{Current: "medium"}
	if got, _ := s.Decide(Verdict{Level: "low"}); got != "medium" {
		t.Fatalf("first verdict changed level to %q", got)
	}
	if got, why := s.Decide(Verdict{Level: "low", Why: "simple task"}); got != "low" || why != "simple task" {
		t.Fatalf("second verdict = %q, %q", got, why)
	}
}

func TestStickyDifferentVerdictResetsStreak(t *testing.T) {
	s := Sticky{Current: "medium"}
	s.Decide(Verdict{Level: "low"})
	if got, _ := s.Decide(Verdict{Level: "high"}); got != "medium" {
		t.Fatalf("changed verdict applied %q", got)
	}
	if got, _ := s.Decide(Verdict{Level: "high"}); got != "high" {
		t.Fatalf("second high verdict applied %q", got)
	}
}

func TestStickyKeepClearsStreak(t *testing.T) {
	s := Sticky{Current: "medium"}
	s.Decide(Verdict{Level: "low"})
	s.Decide(Verdict{Why: "keep"})
	s.Decide(Verdict{Level: "low"})
	if got, _ := s.Decide(Verdict{Level: "low"}); got != "low" {
		t.Fatalf("level did not apply after two new verdicts: %q", got)
	}
}
