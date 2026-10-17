package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRetryableClassification(t *testing.T) {
	yes := []error{
		&HTTPError{Status: 500}, &HTTPError{Status: 503}, &HTTPError{Status: 429}, &HTTPError{Status: 408}, &HTTPError{Status: 425},
		io.ErrUnexpectedEOF, errors.New("connection reset by peer"), errors.New("overloaded"), fmt.Errorf("wrap: %w", &HTTPError{Status: 502}),
	}
	no := []error{
		nil, context.Canceled, &HTTPError{Status: 400}, &HTTPError{Status: 401}, &HTTPError{Status: 403}, &HTTPError{Status: 404}, &HTTPError{Status: 422},
		permanentError{errors.New("bad request body")},
	}
	for _, e := range yes {
		if ok, _ := Retryable(e); !ok {
			t.Errorf("%v must be retryable", e)
		}
	}
	for _, e := range no {
		if ok, _ := Retryable(e); ok {
			t.Errorf("%v must not be retryable", e)
		}
	}
}

func TestPostReportsStatusAndRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		http.Error(w, "busy", http.StatusTooManyRequests)
	}))
	defer srv.Close()
	_, err := post(context.Background(), Provider{BaseURL: srv.URL}, "/x", map[string]string{}, nil)
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 429 || he.RetryAfter != 7*time.Second || he.Body != "busy" {
		t.Fatalf("%#v", err)
	}
	if ok, after := Retryable(err); !ok || after != 7*time.Second {
		t.Fatal(ok, after)
	}
}

func TestParseRetryAfter(t *testing.T) {
	if parseRetryAfter("") != 0 || parseRetryAfter("abc") != 0 || parseRetryAfter("3") != 3*time.Second {
		t.Fatal("seconds")
	}
	d := parseRetryAfter(time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat))
	if d < 25*time.Second || d > 31*time.Second {
		t.Fatal(d)
	}
}
