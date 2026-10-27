// Package cli implements the meno command groups; this file is the API over the
// engine's protocol. An agent's shell call reaches its owner through the
// descriptors (MENO_FD) that the owner passed to the call; a command typed by a
// human runs on an engine started for that one command (see NewLocal).
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"meno/internal/api"
	"meno/internal/callfd"
	"meno/internal/client"
)

// OwnerAPI carries the connection to the owner and, inside an agent call, its capability.
type OwnerAPI struct {
	c                          *client.Client
	home, session, token, call string
}

// New creates an adapter over c. session, token and call may be empty outside
// an agent call (the human commands do not need them).
func New(home string, c *client.Client, session, token, call string) *OwnerAPI {
	if token != "" || call != "" {
		own := *c // the caller's client stays free of this call's credentials
		own.HTTP = &http.Client{Transport: authTransport{base: c.HTTP.Transport, token: token, call: call}}
		c = &own
	}
	return &OwnerAPI{c: c, home: home, session: session, token: token, call: call}
}

// FromEnv builds the adapter from the environment of a bash call.
func FromEnv() *OwnerAPI {
	return New(Home(), client.New(callfd.Dial()), os.Getenv("MENO_SESSION"), os.Getenv("MENO_TOKEN"), os.Getenv("MENO_CALL"))
}

// InsideCall reports whether this process was started by an agent call.
func InsideCall() bool { return os.Getenv("MENO_FD") != "" && os.Getenv("MENO_TOKEN") != "" }

// Home is MENO_HOME, or ~/.meno.
func Home() string {
	if h := os.Getenv("MENO_HOME"); h != "" {
		return h
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return ".meno"
	}
	return filepath.Join(h, ".meno")
}

type authTransport struct {
	base        http.RoundTripper
	token, call string
}

func (a authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	q := r.Clone(r.Context())
	if a.token != "" {
		q.Header.Set("X-Meno-Token", a.token)
	}
	if a.call != "" {
		q.Header.Set("X-Meno-Call", a.call)
	}
	return a.base.RoundTrip(q)
}

// Home is the MENO_HOME the adapter talks to.
func (a *OwnerAPI) Home() string { return a.home }

// resource calls /v1/sessions/{session}/{group}[/{op}] and decodes the data.
func (a *OwnerAPI) resource(ctx context.Context, method, group, op, query string, in, out any) error {
	if a.session == "" {
		return errors.New("this command works inside an agent call (MENO_SESSION is not set)")
	}
	var raw json.RawMessage
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		raw = b
	}
	p := "/v1/sessions/" + url.PathEscape(a.session) + "/" + group
	if op != "" {
		p += "/" + url.PathEscape(op)
	}
	r, err := a.c.Resource(ctx, method, p, query, api.ResourceRequest{Data: raw, Token: a.token, Call: a.call})
	if err != nil {
		return wrap(err)
	}
	if out == nil || len(r.Data) == 0 || string(r.Data) == "null" {
		return nil
	}
	return json.Unmarshal(r.Data, out)
}

func (a *OwnerAPI) post(group, op string, in, out any) error {
	return a.resource(context.Background(), http.MethodPost, group, op, "", in, out)
}

func (a *OwnerAPI) get(group, op, query string, out any) error {
	return a.resource(context.Background(), http.MethodGet, group, op, query, nil, out)
}

// wrap turns a daemon error into a APIError so the exit code is right.
func wrap(err error) error {
	var remote *client.RemoteError
	if errors.As(err, &remote) {
		return &APIError{Code: remote.Code, Message: remote.Message}
	}
	return err
}
