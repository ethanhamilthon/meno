package web

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func local() *Service { return &Service{Policy: Policy{AllowLocal: true}} }

func TestFetchBlocksLocalByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("secret")) }))
	defer srv.Close()
	_, err := (&Service{}).Fetch(context.Background(), srv.URL, FetchOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
}

func TestPolicyAddresses(t *testing.T) {
	deny := []string{"127.0.0.1", "::1", "10.1.2.3", "192.168.0.5", "172.16.0.1", "169.254.1.1", "169.254.169.254", "0.0.0.0", "::ffff:127.0.0.1", "fe80::1", "fc00::1"}
	for _, s := range deny {
		if (Policy{}).Check(netip.MustParseAddr(s)) == nil {
			t.Errorf("%s should be blocked", s)
		}
	}
	if (Policy{}).Check(netip.MustParseAddr("93.184.216.34")) != nil {
		t.Error("public address must pass")
	}
	// Metadata stays blocked even with the local opt-in.
	if (Policy{AllowLocal: true}).Check(netip.MustParseAddr("169.254.169.254")) == nil {
		t.Error("metadata must stay blocked")
	}
}

func TestFetchRedirectToMetadataIsBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()
	_, err := local().Fetch(context.Background(), srv.URL, FetchOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
}

func TestFetchRedirectLoopStops(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, r.URL.Path+"x", 302) }))
	defer srv.Close()
	if _, err := local().Fetch(context.Background(), srv.URL, FetchOptions{Redirects: 3}); err == nil || !strings.Contains(err.Error(), "redirects") {
		t.Fatalf("got %v", err)
	}
}

func TestFetchHTMLToText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><head><title>Hi &amp; bye</title><style>x{}</style></head><body><script>evil()</script><h1>Head</h1><p>One<br>two &lt;3</p></body></html>`))
	}))
	defer srv.Close()
	p, err := local().Fetch(context.Background(), srv.URL, FetchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Hi & bye" || strings.Contains(p.Text, "evil") || !strings.Contains(p.Text, "two <3") {
		t.Fatalf("page %+v", p)
	}
	if p.FinalURL == "" || p.URL != srv.URL {
		t.Fatalf("urls %+v", p)
	}
}

// The size limit counts decoded bytes, so a small gzip bomb is cut off.
func TestFetchGzipBodyIsBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Encoding", "gzip")
		var b bytes.Buffer
		z := gzip.NewWriter(&b)
		z.Write(bytes.Repeat([]byte("a"), 5<<20))
		z.Close()
		w.Write(b.Bytes())
	}))
	defer srv.Close()
	p, err := local().Fetch(context.Background(), srv.URL, FetchOptions{MaxBytes: 1000, MaxText: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Text) > 500 || !p.Truncated || p.Notice == "" {
		t.Fatalf("len=%d truncated=%v", len(p.Text), p.Truncated)
	}
}

func TestFetchTimeoutAndContentType(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	start := time.Now()
	if _, err := local().Fetch(context.Background(), slow.URL, FetchOptions{Timeout: 200 * time.Millisecond}); err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("timeout not enforced: %v", err)
	}
	bin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{0x89, 'P'})
	}))
	defer bin.Close()
	if _, err := local().Fetch(context.Background(), bin.URL, FetchOptions{}); err == nil || !strings.Contains(err.Error(), "not text") {
		t.Fatalf("got %v", err)
	}
}

func TestFetchInvalidEncodingAndSchemes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok \xff\xfe bytes"))
	}))
	defer srv.Close()
	p, err := local().Fetch(context.Background(), srv.URL, FetchOptions{})
	if err != nil || !strings.Contains(p.Text, "\uFFFD") {
		t.Fatalf("%+v %v", p, err)
	}
	for _, u := range []string{"file:///etc/passwd", "ftp://x/y", "javascript:alert(1)", "/relative", ""} {
		if _, err := local().Fetch(context.Background(), u, FetchOptions{}); err == nil {
			t.Errorf("%q should be refused", u)
		}
	}
}

// No credentials of ours reach the target site.
func TestFetchSendsNoCredentials(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Set-Cookie", "a=b")
		w.Write([]byte("x"))
	}))
	defer srv.Close()
	s := local()
	for i := 0; i < 2; i++ {
		if _, err := s.Fetch(context.Background(), srv.URL, FetchOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if got.Get("Authorization") != "" || got.Get("Cookie") != "" {
		t.Fatalf("credentials leaked: %v", got)
	}
}
