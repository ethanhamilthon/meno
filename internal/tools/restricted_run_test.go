package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunRestrictedUsesExecutableAndNeverRunsSecondProgram(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "meno")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "second-ran")
	options := RunOptions{Dir: dir, OutputDir: filepath.Join(dir, "outputs"), CallID: "valid", Env: []string{"PATH=" + dir}}
	call, err := RunRestricted("meno help", binary, []string{"help"}, options)
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
	if _, err := RunRestricted(command, binary, []string{"help"}, options); err == nil || err.Error() != RestrictedCommandError {
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
