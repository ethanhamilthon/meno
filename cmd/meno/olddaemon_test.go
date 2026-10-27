package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnOldDaemonBlocksStartup(t *testing.T) {
	home := t.TempDir()
	if err := checkOldDaemon(home); err != nil {
		t.Fatalf("no daemon: %v", err)
	}
	sock := oldDaemonSocket(home)
	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(sock) })
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkOldDaemon(home); err == nil || !strings.Contains(err.Error(), "older meno") {
		t.Fatalf("a live daemon must block: %v", err)
	}
	l.Close() // the socket file stays, nobody listens: stale
	if err := checkOldDaemon(home); err != nil {
		t.Fatalf("a stale socket must not block: %v", err)
	}
}
