package client

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/daemon"
)

func TestStopConfirmedAbsentAndCleanupTimeout(t *testing.T) {
	home := t.TempDir()
	f, err := StopConfirmed(context.Background(), home, true)
	if err != nil {
		t.Fatal(err)
	}
	// No socket is not enough: held lifetime ownership still blocks confirmation.
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if _, err := StopConfirmed(ctx, home, true); err == nil || !strings.Contains(err.Error(), "cleanup") {
		t.Fatalf("error %v", err)
	}
	f.Close()
	if _, err := os.Stat(daemon.SocketPath(home)); !os.IsNotExist(err) {
		t.Fatalf("updater started daemon: %v", err)
	}
}

func TestStopConfirmedRefusesLegacyAndServerErrors(t *testing.T) {
	for _, tc := range []struct {
		name             string
		protocol, status int
		want             string
	}{
		{"legacy", 3, 200, "cannot safely stop"},
		{"future", api.Protocol + 1, 200, "cannot safely stop"},
		{"forbidden", api.Protocol, 403, "denied"},
		{"conflict", api.Protocol, 409, "finish work"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			os.MkdirAll(home+"/run", 0700)
			listener, err := net.Listen("unix", daemon.SocketPath(home))
			if err != nil {
				t.Fatal(err)
			}
			shutdowns := make(chan struct{}, 1)
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/hello" {
					json.NewEncoder(w).Encode(api.Hello{Protocol: tc.protocol})
					return
				}
				shutdowns <- struct{}{}
				w.WriteHeader(tc.status)
				json.NewEncoder(w).Encode(api.ErrorEnvelope{Error: api.ErrorBody{Code: api.CodeConflict, Message: tc.want}})
			})}
			go server.Serve(listener)
			defer server.Close()
			if _, err := StopConfirmed(context.Background(), home, true); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v", err)
			}
			if tc.name == "legacy" || tc.name == "future" {
				select {
				case <-shutdowns:
					t.Fatal("legacy shutdown requested")
				default:
				}
			}
		})
	}
}

func TestStopConfirmedUnreachableSocketIsNotSuccess(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(home+"/run", 0700)
	os.WriteFile(daemon.SocketPath(home), []byte("not a socket"), 0600)
	// macOS/Linux can return ECONNREFUSED for a regular-file socket path. Treat
	// that like a stale socket only when lifetime ownership is actually free.
	lock, err := daemon.AcquireOwnership(context.Background(), home, "daemon.lock")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if _, err := StopConfirmed(ctx, home, true); err == nil {
		t.Fatal("reported stopped with lifecycle ownership held")
	}
}
