package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/cliproxy"
)

// fakeHelper stands in for cliproxyapi: management API for logins and accounts.
type fakeHelper struct {
	mu       sync.Mutex
	accounts []map[string]any
	polls    atomic.Int32
	okAfter  int32
	callback []string
}

func (h *fakeHelper) serve(key string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+key {
			w.WriteHeader(401)
			return
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		switch r.URL.Path {
		case "/v0/management/latest-version":
			_, _ = w.Write([]byte(`{"latest-version":"v8.0.33"}`))
		case "/v8/management/oauth/auth-url":
			if r.URL.Query().Get("provider") == "xai" {
				_, _ = w.Write([]byte(`{"flow":"device","state":"st-dev","status":"ok","url":"https://x.ai/device","user_code":"AB-12","expires_in":600}`))
				return
			}
			_, _ = w.Write([]byte(`{"state":"st-1","status":"ok","url":"https://claude.ai/oauth/authorize?x=1"}`))
		case "/v0/management/get-auth-status":
			if h.polls.Add(1) >= h.okAfter {
				h.accounts = append(h.accounts, map[string]any{"name": "claude-1.json", "provider": "claude", "email": "me@example.com"})
				_, _ = w.Write([]byte(`{"status":"ok"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"wait"}`))
		case "/v0/management/auth-files":
			if r.Method == http.MethodDelete {
				name := r.URL.Query().Get("name")
				for i, a := range h.accounts {
					if a["name"] == name {
						h.accounts = append(h.accounts[:i], h.accounts[i+1:]...)
						break
					}
				}
				_, _ = w.Write([]byte(`{"status":"ok"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"files": h.accounts})
		case "/v0/management/oauth-callback":
			var b struct {
				URL string `json:"redirect_url"`
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			h.callback = append(h.callback, b.URL)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v0/management/auth-files/status":
			var b struct {
				Name     string `json:"name"`
				Disabled bool   `json:"disabled"`
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			for _, a := range h.accounts {
				if a["name"] == b.Name {
					a["disabled"] = b.Disabled
				}
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	})
}

// loginApp is an app whose OAuth helper is the fake server (so it counts as installed and running).
func loginApp(t *testing.T) (*App, tcell.SimulationScreen, *fakeHelper, *[]string) {
	t.Helper()
	a, scr := newTestApp(t)
	h := &fakeHelper{okAfter: 2}
	key := a.oauth.State().ManagementKey
	srv := httptest.NewServer(h.serve(key))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	a.oauth.Update(func(s *cliproxy.State) { s.Port, s.Version = port, "v8.0.33" })
	bin := a.oauth.BinaryPath("v8.0.33")
	_ = os.MkdirAll(filepath.Dir(bin), 0o755)
	_ = os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755)
	opened := &[]string{}
	a.openURL = func(u string) error { *opened = append(*opened, u); return nil }
	old := pollEvery
	pollEvery = 20 * time.Millisecond
	t.Cleanup(func() { pollEvery = old })
	return a, scr, h, opened
}
