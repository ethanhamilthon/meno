package cliproxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

// fakeHelper answers like the management API of the helper and records requests.
type fakeHelper struct {
	reqs []string
	body map[string]any
}

func newAPITest(t *testing.T) (*Manager, *fakeHelper) {
	h := &fakeHelper{}
	m := New(t.TempDir())
	key := m.State().ManagementKey
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+key {
			w.WriteHeader(401)
			return
		}
		h.reqs = append(h.reqs, r.Method+" "+r.URL.RequestURI())
		raw, _ := io.ReadAll(r.Body)
		h.body = nil
		_ = json.Unmarshal(raw, &h.body)
		switch r.URL.Path {
		case "/v8/management/oauth/auth-url":
			if r.URL.Query().Get("provider") == "xai" {
				_, _ = w.Write([]byte(`{"flow":"device","state":"xai-1","status":"ok","url":"https://x.ai/device","user_code":"AB-12","expires_in":1800}`))
				return
			}
			_, _ = w.Write([]byte(`{"state":"s1","status":"ok","url":"https://claude.ai/oauth"}`))
		case "/v0/management/get-auth-status":
			_, _ = w.Write([]byte(`{"status":"wait"}`))
		case "/v0/management/auth-files":
			if r.Method == http.MethodDelete {
				_, _ = w.Write([]byte(`{"status":"ok"}`))
				return
			}
			_, _ = w.Write([]byte(`{"files":[{"name":"codex-1.json","provider":"codex","email":"a@b.c","disabled":false,"id_token":{"plan_type":"plus"}},{"name":"kimi-2.json","provider":"kimi","label":"kim","disabled":true,"failed":2,"status_message":"disabled (invalid grant)"}]}`))
		case "/v0/management/auth-files/status", "/v0/management/oauth-callback":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	m.Update(func(s *State) { s.Port = port })
	return m, h
}

func TestStartLoginRedirectAndDevice(t *testing.T) {
	m, h := newAPITest(t)
	l, err := m.StartLogin(context.Background(), "claude")
	if err != nil || l.Device || l.State != "s1" || l.URL == "" {
		t.Fatalf("%+v %v", l, err)
	}
	if h.reqs[0] != "GET /v8/management/oauth/auth-url?is_webui=true&provider=claude" {
		t.Fatalf("request %q", h.reqs[0])
	}
	d, err := m.StartLogin(context.Background(), "xai")
	if err != nil || !d.Device || d.UserCode != "AB-12" || d.ExpiresIn != 1800*time.Second {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestLoginStatusAndCallback(t *testing.T) {
	m, h := newAPITest(t)
	st, _, err := m.LoginStatus(context.Background(), "s1")
	if err != nil || st != "wait" {
		t.Fatalf("%q %v", st, err)
	}
	if err := m.SubmitCallback(context.Background(), "http://localhost:54545/callback?code=c&state=s1"); err != nil {
		t.Fatal(err)
	}
	if h.body["redirect_url"] == "" || len(h.body) != 1 {
		t.Fatalf("body %v", h.body)
	}
}

func TestAccountsSetDisabledLogout(t *testing.T) {
	m, h := newAPITest(t)
	list, err := m.Accounts(context.Background())
	if err != nil || len(list) != 2 {
		t.Fatalf("%+v %v", list, err)
	}
	if a := list[0]; a.Service != "ChatGPT / Codex" || a.Label != "a@b.c" || a.Plan != "plus" || a.Disabled || a.File != "codex-1.json" {
		t.Fatalf("%+v", a)
	}
	if a := list[1]; a.Service != "Kimi" || a.Label != "kim" || !a.Disabled || a.Failed != 2 || a.Note != "disabled (invalid grant)" {
		t.Fatalf("%+v", a)
	}
	if err := m.SetDisabled(context.Background(), "codex-1.json", true); err != nil || h.body["name"] != "codex-1.json" || h.body["disabled"] != true {
		t.Fatalf("%v %v", err, h.body)
	}
	if err := m.Logout(context.Background(), "codex-1.json"); err != nil || h.reqs[len(h.reqs)-1] != "DELETE /v0/management/auth-files?name=codex-1.json" {
		t.Fatalf("%v %v", err, h.reqs)
	}
}

func TestTransportFailureIsHelperDown(t *testing.T) {
	m := New(t.TempDir())
	m.Update(func(s *State) { s.Port = freePort() })
	_, err := m.Accounts(context.Background())
	if !errors.Is(err, ErrHelperDown) {
		t.Fatalf("error = %v, want ErrHelperDown", err)
	}
}

func TestWrongKeyIsAnError(t *testing.T) {
	m, _ := newAPITest(t)
	m.Update(func(s *State) { s.ManagementKey = "other" })
	if _, err := m.Accounts(context.Background()); err == nil {
		t.Fatal("a wrong management key must fail")
	}
}

func TestWaitReadyEndsWithTheContext(t *testing.T) {
	m := New(t.TempDir())
	m.Update(func(s *State) { s.Port = freePort() }) // nothing listens there
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if err := m.WaitReady(ctx); err == nil {
		t.Fatal("no server: WaitReady must give up")
	}
}

func TestServices(t *testing.T) {
	if len(Services) != 6 {
		t.Fatal("six OAuth services")
	}
	if s, ok := ServiceOf("anthropic"); !ok || s.ID != "claude" {
		t.Fatal("claude files belong to the claude login")
	}
	if _, ok := ServiceByID("nope"); ok {
		t.Fatal("unknown id")
	}
}
