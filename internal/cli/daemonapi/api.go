// Package daemonapi implements cli.API over the daemon's unix-socket HTTP API.
// An agent's shell call reaches the daemon with the session id, token, call id
// and socket path that the daemon put in the environment of the call.
package daemonapi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"meno/internal/cli"
	"meno/internal/client"
	"meno/internal/proto"
)

// API carries the daemon connection and, inside an agent call, its capability.
type API struct {
	c                          *client.Client
	home, session, token, call string
}

// New creates an adapter for the daemon at socket. session, token and call
// may be empty outside an agent call (the human commands do not need them).
func New(home, socket, session, token, call string) *API {
	if socket == "" {
		socket = filepath.Join(home, "run", "meno.sock")
	}
	c := client.New(home)
	c.Socket = socket
	base := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	c.HTTP = &http.Client{Transport: authTransport{base: base, token: token, call: call}}
	return &API{c: c, home: home, session: session, token: token, call: call}
}

// FromEnv builds the adapter from the environment of a bash call. Outside a
// call it falls back to MENO_HOME (or ~/.meno) and no session.
func FromEnv() (*API, error) {
	home := os.Getenv("MENO_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		home = filepath.Join(h, ".meno")
	}
	return New(home, os.Getenv("MENO_SOCK"), os.Getenv("MENO_SESSION"), os.Getenv("MENO_TOKEN"), os.Getenv("MENO_CALL")), nil
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
func (a *API) Home() string { return a.home }

// resource calls /v1/sessions/{session}/{group}[/{op}] and decodes the data.
func (a *API) resource(ctx context.Context, method, group, op, query string, in, out any) error {
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
	r, err := a.c.Resource(ctx, method, p, query, proto.ResourceRequest{Data: raw, Token: a.token, Call: a.call})
	if err != nil {
		return wrap(err)
	}
	if out == nil || len(r.Data) == 0 || string(r.Data) == "null" {
		return nil
	}
	return json.Unmarshal(r.Data, out)
}

func (a *API) post(group, op string, in, out any) error {
	return a.resource(context.Background(), http.MethodPost, group, op, "", in, out)
}

func (a *API) get(group, op, query string, out any) error {
	return a.resource(context.Background(), http.MethodGet, group, op, query, nil, out)
}

// wrap turns a daemon error into a cli.APIError so the exit code is right.
func wrap(err error) error {
	var remote *client.RemoteError
	if errors.As(err, &remote) {
		return &cli.APIError{Code: remote.Code, Message: remote.Message}
	}
	return err
}
