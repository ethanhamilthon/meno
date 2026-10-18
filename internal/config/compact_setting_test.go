package config

import "testing"

func TestCompactPercentIsAdjustedInStepsAndBounded(t *testing.T) {
	s := DefaultSettings()
	if s.NumFor("compact_percent") != 80 || NumOption("compact_percent") != "percent" {
		t.Fatalf("%d %q", s.NumFor("compact_percent"), NumOption("compact_percent"))
	}
	if got := s.AdjustNum("compact_percent", 1); got != 85 {
		t.Fatalf("one step up from 80: %d", got)
	}
	for i := 0; i < 10; i++ {
		s.AdjustNum("compact_percent", 1)
	}
	if s.CompactPercent != MaxCompactPercent {
		t.Fatalf("upper bound: %d", s.CompactPercent)
	}
	for i := 0; i < 30; i++ {
		s.AdjustNum("compact_percent", -1)
	}
	if s.CompactPercent != MinCompactPercent {
		t.Fatalf("lower bound: %d", s.CompactPercent)
	}
	// The row has one choice, so cycling it never changes the number.
	before := s.CompactPercent
	s.Cycle("compact_percent", 1)
	if s.CompactPercent != before {
		t.Fatal("cycling changed the threshold")
	}
}
