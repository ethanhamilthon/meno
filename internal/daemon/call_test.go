package daemon

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"

	"meno/internal/api"
	"meno/internal/callfd"
)

// tokenSpy records who the resource handler thinks is asking.
type tokenSpy struct {
	NotImplementedBackend
	got chan api.ResourceRequest
}

func (b tokenSpy) Resource(_ context.Context, _, _, _ string, q api.ResourceRequest) (api.ResourceResponse, error) {
	b.got <- q
	return api.ResourceResponse{}, nil
}

// A connection of a call speaks as that call whatever the client sends.
func TestCallConnectionIsBoundToItsToken(t *testing.T) {
	spy := tokenSpy{got: make(chan api.ResourceRequest, 1)}
	s := New(Options{Backend: spy})
	// Every dial is a new connection of the same call, as the descriptors give it.
	dial := func(context.Context, string, string) (net.Conn, error) {
		client, server, err := callfd.Pair()
		if err != nil {
			return nil, err
		}
		go s.ServeCall(server, "the-call-token", "call-7")
		return client, nil
	}
	c := &http.Client{Transport: &http.Transport{DialContext: dial}}
	req, _ := http.NewRequest("POST", "http://x/v1/sessions/s1/tasks/list", strings.NewReader(`{}`))
	req.Header.Set("X-Meno-Token", "forged")
	req.Header.Set("Authorization", "Bearer also-forged")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	q := <-spy.got
	if q.Token != "the-call-token" || q.Call != "call-7" {
		t.Fatalf("the request spoke as %+v", q)
	}
	// A request without any credentials is still the call, not a human.
	req, _ = http.NewRequest("POST", "http://x/v1/sessions/s1/tasks/list", strings.NewReader(`{}`))
	resp, err = c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if q = <-spy.got; q.Token != "the-call-token" {
		t.Fatalf("an anonymous request spoke as %+v", q)
	}
}
