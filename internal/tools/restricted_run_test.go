package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunRestrictedUsesExecutableAndNeverRunsSecondProgram(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "meno")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "second-ran")
	ctx := context.Background()
	options := RunOptions{Dir: dir, OutputDir: filepath.Join(dir, "outputs"), CallID: "valid", Timeout: time.Second, Env: []string{"PATH=" + dir}}
	call, err := RunRestricted(ctx, "meno help", binary, []string{"help"}, options)
	if err != nil {
		t.Fatal(err)
	}
	result, err := call.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("result = %#v", result)
	}
	options.CallID = "denied"
	command := "meno help; touch " + marker
	if _, err := RunRestricted(ctx, command, binary, []string{"help"}, options); err == nil || err.Error() != RestrictedCommandError {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("second program ran: %v", err)
	}
}

func TestRestrictedRequiresAbsoluteDaemonExecutable(t *testing.T) {
	if _, err := ParseRestrictedCommand("meno help", "meno", []string{"help"}); err == nil {
		t.Fatal("accepted PATH-resolved executable")
	}
}
