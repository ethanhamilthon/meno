package ui

import "testing"

func TestCompactCount(t *testing.T) {
	for n, want := range map[int]string{
		-5: "0", 0: "0", 999: "999", 1000: "1K", 1234: "1.2K", 9949: "9.9K", 12345: "12K", 120000: "120K",
		999499: "999K", 999500: "1M", 1500000: "1.5M", 12000000: "12M",
	} {
		if got := CompactCount(n); got != want {
			t.Errorf("CompactCount(%d) = %q, want %q", n, got, want)
		}
	}
}
