package cliproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrHelperDown reports that the local OAuth helper cannot be reached.
var ErrHelperDown = errors.New("OAuth helper is not responding")

func (m *Manager) client() *http.Client {
	if m.HTTP != nil {
		return m.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// call sends a request to the management API of the local server.
func (m *Manager) call(ctx context.Context, method, path string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, m.BaseURL()+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.State().ManagementKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.client().Do(req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "connection refused") || strings.Contains(strings.ToLower(err.Error()), "dial tcp") {
			return fmt.Errorf("%w: %v", ErrHelperDown, err)
		}
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Error == "" {
			e.Error = resp.Status
		}
		return errors.New(e.Error)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// Ready reports whether the server answers.
func (m *Manager) Ready(ctx context.Context) bool {
	return m.call(ctx, http.MethodGet, "/v0/management/latest-version", nil, nil) == nil ||
		m.call(ctx, http.MethodGet, "/v0/management/auth-files", nil, nil) == nil
}

// WaitReady waits until the server answers or the context ends.
func (m *Manager) WaitReady(ctx context.Context) error {
	for {
		c, cancel := context.WithTimeout(ctx, 2*time.Second)
		ok := m.Ready(c)
		cancel()
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// Login is a started OAuth login. With Device set the user types UserCode on
// the page at URL; otherwise the browser comes back by itself.
type Login struct {
	URL       string
	State     string
	UserCode  string
	Device    bool
	ExpiresIn time.Duration
}

// StartLogin asks for the login page of a service (Service.ID). The helper
// listens for the browser's return on the usual local port while the login
// lasts.
func (m *Manager) StartLogin(ctx context.Context, service string) (Login, error) {
	var r struct {
		URL       string `json:"url"`
		State     string `json:"state"`
		UserCode  string `json:"user_code"`
		Flow      string `json:"flow"`
		ExpiresIn int    `json:"expires_in"`
		Error     string `json:"error"`
	}
	q := url.Values{"provider": {service}, "is_webui": {"true"}}
	if err := m.call(ctx, http.MethodGet, "/v8/management/oauth/auth-url?"+q.Encode(), nil, &r); err != nil {
		return Login{}, err
	}
	if r.URL == "" || r.State == "" {
		return Login{}, errors.New("the helper returned no login page")
	}
	return Login{URL: r.URL, State: r.State, UserCode: r.UserCode, Device: r.Flow == "device", ExpiresIn: time.Duration(r.ExpiresIn) * time.Second}, nil
}

// LoginStatus is "wait", "ok" or "error" (then msg says why).
func (m *Manager) LoginStatus(ctx context.Context, state string) (status, msg string, err error) {
	var r struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	q := url.Values{"state": {state}}
	if err := m.call(ctx, http.MethodGet, "/v0/management/get-auth-status?"+q.Encode(), nil, &r); err != nil {
		return "", "", err
	}
	return r.Status, r.Error, nil
}

// SubmitCallback hands the helper the address the browser was sent to, for
// the case where the browser cannot reach this machine (a remote shell). The
// address carries the state that tells the helper which login it belongs to.
func (m *Manager) SubmitCallback(ctx context.Context, redirectURL string) error {
	return m.call(ctx, http.MethodPost, "/v0/management/oauth-callback", map[string]string{"redirect_url": redirectURL}, nil)
}

// Account is one logged-in account.
type Account struct {
	File     string // name of the auth file: the id for SetDisabled and Logout
	Provider string // provider name in the file ("codex")
	Service  string // label of the service
	Label    string // e-mail or account name
	Plan     string
	Disabled bool
	Failed   int
	Note     string // helper status text, for example why a token was disabled
}

// Accounts lists the accounts of the helper.
func (m *Manager) Accounts(ctx context.Context) ([]Account, error) {
	var r struct {
		Files []struct {
			Name          string `json:"name"`
			Provider      string `json:"provider"`
			Label         string `json:"label"`
			Email         string `json:"email"`
			Account       string `json:"account"`
			Disabled      bool   `json:"disabled"`
			StatusMessage string `json:"status_message"`
			Failed        int    `json:"failed"`
			IDToken       struct {
				Plan string `json:"plan_type"`
			} `json:"id_token"`
		} `json:"files"`
	}
	if err := m.call(ctx, http.MethodGet, "/v0/management/auth-files", nil, &r); err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(r.Files))
	for _, f := range r.Files {
		label := firstNonEmpty(f.Email, f.Label, f.Account, f.Name)
		svc := f.Provider
		if s, ok := ServiceOf(f.Provider); ok {
			svc = s.Label
		}
		out = append(out, Account{File: f.Name, Provider: f.Provider, Service: svc, Label: label, Plan: f.IDToken.Plan, Disabled: f.Disabled, Failed: f.Failed, Note: f.StatusMessage})
	}
	return out, nil
}

// SetDisabled turns an account off or on without logging it out.
func (m *Manager) SetDisabled(ctx context.Context, file string, disabled bool) error {
	return m.call(ctx, http.MethodPatch, "/v0/management/auth-files/status", map[string]any{"name": file, "disabled": disabled}, nil)
}

// Logout removes the account and its token.
func (m *Manager) Logout(ctx context.Context, file string) error {
	q := url.Values{"name": {file}}
	if err := m.call(ctx, http.MethodDelete, "/v0/management/auth-files?"+q.Encode(), nil, nil); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	return nil
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}
