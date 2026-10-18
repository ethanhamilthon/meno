package safe

import (
	"strings"
	"testing"
	"time"
)

func TestCallReturnsPanicAsError(t *testing.T) {
	err := Call(func() { panic("boom") })
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want panic error, got %v", err)
	}
	if err := Call(func() {}); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestGoReportsPanicWithoutCrashing(t *testing.T) {
	got := make(chan error, 1)
	Go(func() { var m map[string]int; m["x"] = 1 }, func(err error) { got <- err })
	select {
	case err := <-got:
		if err == nil || !strings.Contains(err.Error(), "nil map") {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("onPanic was not called")
	}
	done := make(chan struct{})
	Go(func() { close(done) }, func(error) { t.Error("unexpected panic") })
	<-done
}
