package daemonapi

import (
	"errors"
	"testing"
)

func TestInterruptedErrorIsDistinct(t *testing.T) {
	if errors.Is(errInterrupted, errStop) {
		t.Fatal("interruption must not look like a normal stop")
	}
}
