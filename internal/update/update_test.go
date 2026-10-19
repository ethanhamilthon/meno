package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		cur, latest string
		want        bool
	}{
		{"0.1.1", "v0.1.2", true}, {"0.1.1", "v0.2.0", true}, {"0.9.0", "v0.10.0", true},
		{"0.1.1", "v0.1.1", false}, {"0.2.0", "v0.1.9", false},
		{"dev", "v1.0.0", false}, {"0.1.1", "nightly", false}, {"0.0.0-ci", "v0.0.1", true},
	} {
		if got := Newer(c.cur, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.cur, c.latest, got)
		}
	}
}

func archive(t *testing.T, body string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "meno", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	_, _ = tw.Write([]byte(body))
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func release(t *testing.T, tag, binary, sumOverride string) *httptest.Server {
	arc := archive(t, binary)
	h := sha256.Sum256(arc)
	sum := hex.EncodeToString(h[:])
	if sumOverride != "" {
		sum = sumOverride
	}
	name := fmt.Sprintf("meno_%s_linux_amd64.tar.gz", strings.TrimPrefix(tag, "v"))
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name":%q}`, tag)
	})
	mux.HandleFunc("/dl/SHA256SUMS", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintf(w, "%s  %s\n", sum, name) })
	mux.HandleFunc("/dl/"+name, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(arc) })
	return httptest.NewServer(mux)
}

func TestInstallReplacesTheExecutable(t *testing.T) {
	srv := release(t, "v0.2.0", "new binary", "")
	defer srv.Close()
	exe := filepath.Join(t.TempDir(), "meno")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := Client{Repo: "o/r", API: srv.URL, Base: srv.URL + "/dl"}
	tag, err := c.Latest(context.Background())
	if err != nil || tag != "v0.2.0" {
		t.Fatalf("latest: %q %v", tag, err)
	}
	if err := c.install(context.Background(), tag, exe, "linux", "amd64"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "new binary" {
		t.Fatalf("executable: %q", b)
	}
	if st, _ := os.Stat(exe); st.Mode()&0o111 == 0 {
		t.Fatal("not executable")
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(exe), ".meno-update-*")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}

func TestInstallRefusesABadChecksumAndKeepsTheOldBinary(t *testing.T) {
	srv := release(t, "v0.2.0", "new binary", strings.Repeat("0", 64))
	defer srv.Close()
	exe := filepath.Join(t.TempDir(), "meno")
	_ = os.WriteFile(exe, []byte("old"), 0o755)
	c := Client{Repo: "o/r", API: srv.URL, Base: srv.URL + "/dl"}
	err := c.install(context.Background(), "v0.2.0", exe, "linux", "amd64")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v", err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old" {
		t.Fatalf("executable changed: %q", b)
	}
	if err := c.install(context.Background(), "v0.2.0", exe, "linux", "arm64"); err == nil || !strings.Contains(err.Error(), "no build") {
		t.Fatalf("missing platform: %v", err)
	}
}

func TestCheckCachesTheAnswer(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		fmt.Fprint(w, `{"tag_name":"v0.3.0"}`)
	}))
	defer srv.Close()
	c := Client{Repo: "o/r", API: srv.URL}
	home, now := t.TempDir(), time.Now()
	if got := c.Check(context.Background(), home, "0.1.1", now); got != "v0.3.0" {
		t.Fatalf("check: %q", got)
	}
	if got := c.Check(context.Background(), home, "0.1.1", now.Add(time.Hour)); got != "v0.3.0" || hits != 1 {
		t.Fatalf("cached check: %q after %d requests", got, hits)
	}
	if got := c.Check(context.Background(), home, "0.3.0", now.Add(CheckEvery+time.Minute)); got != "" || hits != 2 {
		t.Fatalf("expired check: %q after %d requests", got, hits)
	}
}

func TestCheckIgnoresFailures(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c := Client{Repo: "o/r", API: srv.URL}
	if got := c.Check(context.Background(), t.TempDir(), "0.1.1", time.Now()); got != "" {
		t.Fatalf("check: %q", got)
	}
}

func TestLatestReadsTheReleasePageRedirect(t *testing.T) {
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/o/r/releases/latest" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/o/r/releases/tag/v1.2.3", http.StatusFound)
	}))
	defer web.Close()
	c := Client{Repo: "o/r", Web: web.URL}
	tag, err := c.Latest(context.Background())
	if err != nil || tag != "v1.2.3" {
		t.Fatalf("tag %q err %v", tag, err)
	}
}

func TestLatestFallsBackToTheAPIWhenThePageFails(t *testing.T) {
	web := httptest.NewServer(http.NotFoundHandler())
	defer web.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v2.0.0"}`)
	}))
	defer api.Close()
	// With an API base set the page is skipped, as in the other tests; without
	// one a failing page must leave the API as the way out.
	c := Client{Repo: "o/r", Web: web.URL, API: api.URL}
	if tag, err := c.Latest(context.Background()); err != nil || tag != "v2.0.0" {
		t.Fatalf("tag %q err %v", tag, err)
	}
}
