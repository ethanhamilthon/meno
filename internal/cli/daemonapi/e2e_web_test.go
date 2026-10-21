package daemonapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"meno/internal/config"
)

// Through the real CLI, daemon and engine: a local destination is forbidden
// (exit 3), and the same fetch works once the user opted in.
func TestWebFetchThroughTheDaemonHonorsThePolicy(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><head><title>Local page</title></head><body><p>hello from the site</p></body></html>"))
	}))
	defer site.Close()
	home, _, _, api := start(t, &scripted{})

	code, _, errOut := run(t, api, "web", "fetch", site.URL)
	if code != 3 || !strings.Contains(errOut, "not allowed") {
		t.Fatalf("default policy: exit %d, stderr %q", code, errOut)
	}

	st := config.NewStore(config.Paths{Home: home})
	s, _ := st.LoadCurrentSettings()
	s.WebAllowLocal = true
	if err := config.SaveSettingsFile(st.P.SettingsFile(), s); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := run(t, api, "web", "fetch", site.URL)
	if code != 0 || !strings.Contains(out, "Title: Local page") || !strings.Contains(out, "hello from the site") || !strings.Contains(out, "untrusted") {
		t.Fatalf("opted in: exit %d\nstdout %q\nstderr %q", code, out, errOut)
	}
}

func TestWebSearchThroughTheDaemonNeedsAConfiguredProvider(t *testing.T) {
	_, _, _, api := start(t, &scripted{})
	code, _, errOut := run(t, api, "web", "search", "anything")
	if code == 0 || !strings.Contains(errOut, "not configured") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}
