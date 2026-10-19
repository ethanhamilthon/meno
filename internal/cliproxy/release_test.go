package cliproxy

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
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"meno/internal/update"
)

// fakeRelease serves one GitHub release at a time for the current platform.
type fakeRelease struct {
	mu  sync.Mutex
	tag string
	srv *httptest.Server
	hit int
}

func newFakeRelease(t *testing.T, tag string) *fakeRelease {
	f := &fakeRelease{tag: tag}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeRelease) setTag(tag string) { f.mu.Lock(); f.tag = tag; f.mu.Unlock() }

func (f *fakeRelease) archive() (name string, data []byte) {
	arch := runtime.GOARCH
	if arch == "arm64" {
		arch = "aarch64"
	}
	name = fmt.Sprintf("CLIProxyAPI_%s_%s_%s.tar.gz", strings.TrimPrefix(f.tag, "v"), runtime.GOOS, arch)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\n# " + f.tag + "\n")
	_ = tw.WriteHeader(&tar.Header{Name: "cli-proxy-api", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()
	return name, buf.Bytes()
}

func (f *fakeRelease) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	name, data := f.archive()
	switch {
	case strings.HasSuffix(r.URL.Path, "/releases/latest"):
		f.hit++
		_, _ = fmt.Fprintf(w, `{"tag_name":%q}`, f.tag)
	case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
		sum := sha256.Sum256(data)
		_, _ = fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	case strings.HasSuffix(r.URL.Path, name):
		_, _ = w.Write(data)
	default:
		http.NotFound(w, r)
	}
}

func testManager(t *testing.T, f *fakeRelease) *Manager {
	m := New(t.TempDir())
	m.Release = update.Client{Repo: "x/y", API: f.srv.URL, Base: f.srv.URL}
	return m
}

func TestEnsureInstallsTheNewestRelease(t *testing.T) {
	f := newFakeRelease(t, "v8.0.33")
	m := testManager(t, f)
	var steps []string
	if err := m.Ensure(context.Background(), func(s string) { steps = append(steps, s) }); err != nil {
		t.Fatal(err)
	}
	if !m.Installed() || m.State().Version != "v8.0.33" || len(steps) != 2 {
		t.Fatalf("state %+v steps %v", m.State(), steps)
	}
	info, err := os.Stat(m.BinaryPath("v8.0.33"))
	if err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("binary: %v %v", info, err)
	}
	if err := m.Ensure(context.Background(), nil); err != nil || f.hit != 1 {
		t.Fatalf("a second Ensure must not ask GitHub: hits=%d err=%v", f.hit, err)
	}
}

func TestRefreshDownloadsWithinTheMajorAndActivatesLater(t *testing.T) {
	f := newFakeRelease(t, "v8.0.33")
	m := testManager(t, f)
	now := time.Now()
	if _, err := m.Refresh(context.Background(), now); err != nil || m.State().Version != "v8.0.33" {
		t.Fatalf("first install: %v %+v", err, m.State())
	}
	f.setTag("v8.1.0")
	if out, _ := m.Refresh(context.Background(), now.Add(time.Hour)); out != (Outcome{}) {
		t.Fatalf("within CheckEvery nothing is asked: %+v", out)
	}
	out, err := m.Refresh(context.Background(), now.Add(7*time.Hour))
	if err != nil || out.Downloaded != "v8.1.0" {
		t.Fatalf("outcome %+v err %v", out, err)
	}
	st := m.State()
	if st.Version != "v8.0.33" || st.Pending != "v8.1.0" || !fileExists(m.BinaryPath("v8.1.0")) {
		t.Fatalf("the new version must wait for a restart: %+v", st)
	}
	m.Activate()
	st = m.State()
	if st.Version != "v8.1.0" || st.Pending != "" || !fileExists(m.BinaryPath("v8.0.33")) {
		t.Fatalf("activate: %+v (the old version stays for a way back)", st)
	}
}

func TestRefreshReportsANewMajorWithoutInstalling(t *testing.T) {
	f := newFakeRelease(t, "v8.0.33")
	m := testManager(t, f)
	now := time.Now()
	if _, err := m.Refresh(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	f.setTag("v9.0.0")
	out, err := m.Refresh(context.Background(), now.Add(7*time.Hour))
	if err != nil || out.Major != "v9.0.0" || out.Downloaded != "" {
		t.Fatalf("outcome %+v err %v", out, err)
	}
	if st := m.State(); st.Pending != "" || fileExists(m.BinaryPath("v9.0.0")) {
		t.Fatalf("a new major version must not be installed: %+v", st)
	}
}

func TestUseKeepsOnlyTheLastTwoVersions(t *testing.T) {
	f := newFakeRelease(t, "v8.0.1")
	m := testManager(t, f)
	for _, tag := range []string{"v8.0.1", "v8.0.2", "v8.0.3"} {
		f.setTag(tag)
		if err := m.Install(context.Background(), tag); err != nil {
			t.Fatal(err)
		}
		m.Use(tag)
	}
	if fileExists(m.BinaryPath("v8.0.1")) || !fileExists(m.BinaryPath("v8.0.2")) || !fileExists(m.BinaryPath("v8.0.3")) {
		t.Fatal("expected only v8.0.2 and v8.0.3")
	}
}
