package engine

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"meno/internal/api"
	"meno/internal/daemon"
)

func isCode(err error, code string) bool {
	var a *daemon.APIError
	return errors.As(err, &a) && a.Code == code
}

func TestWebFetchRefusesLocalAddressByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("x")) }))
	defer srv.Close()
	e, _ := fixture(t, &fakeProvider{})
	_, err := e.webResource(context.Background(), []string{"fetch"}, "POST", map[string]any{"url": srv.URL})
	if !isCode(err, api.CodeForbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestWebSearchWithoutConfigurationIsABadRequest(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	_, err := e.webResource(context.Background(), []string{"search"}, "POST", map[string]any{"query": "q"})
	if !isCode(err, api.CodeBadRequest) {
		t.Fatalf("got %v", err)
	}
}

func TestWebSearchUnknownProviderIsReported(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	_, err := e.searchBackend("nope/model")
	if !isCode(err, api.CodeBadRequest) {
		t.Fatalf("got %v", err)
	}
}
