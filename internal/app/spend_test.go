package app

import "testing"

func TestFmtSpendKeepsFourDigits(t *testing.T) {
	for v, want := range map[float64]string{
		0: "$0", 0.0001: "<$0.001", 0.00123: "$0.001", 0.0126: "$0.013", 0.1837: "$0.184", 1.25: "$1.25",
		1.2345: "$1.234", 12.34: "$12.34", 123.46: "$123.5", 1234.6: "$1235", 20: "$20", 0.5: "$0.5",
	} {
		if got := fmtSpend(v); got != want {
			t.Errorf("%v → %q, want %q", v, got, want)
		}
	}
}

func TestStatusUsageDropsFieldsFromTheRight(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Usage.In, a.st.Usage.Out, a.st.Spent = 2_100_000, 35_000, 0.0012
	full := a.statusUsage(1 << 30)
	if full != "2.1m/35k ↺?/? ◫ 0/? $0.001" {
		t.Fatalf("%q", full)
	}
	for width, want := range map[int]string{22: "2.1m/35k ↺?/? ◫ 0/?", 13: "2.1m/35k ↺?/?", 3: "2.1m/35k"} {
		if got := a.statusUsage(width); got != want {
			t.Errorf("width %d: %q, want %q", width, got, want)
		}
	}
}
