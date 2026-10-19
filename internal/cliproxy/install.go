package cliproxy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"meno/internal/update"
)

// CheckEvery is how long a look at the newest release stays fresh.
const CheckEvery = 6 * time.Hour

// asset is the archive of a release for this platform. The helper names its
// arm64 builds aarch64.
func asset(goos, goarch string) update.Asset {
	if goarch == "arm64" {
		goarch = "aarch64"
	}
	return update.Asset{Sums: "checksums.txt", Suffix: "_" + goos + "_" + goarch + ".tar.gz", Binary: binaryName}
}

// Install downloads the release tag into its own directory. The helper in use
// does not change: call Use for that.
func (m *Manager) Install(ctx context.Context, tag string) error {
	return m.installFor(ctx, tag, runtime.GOOS, runtime.GOARCH)
}

func (m *Manager) installFor(ctx context.Context, tag, goos, goarch string) error {
	dest := m.BinaryPath(tag)
	if fileExists(dest) {
		return nil
	}
	return m.Release.InstallAsset(ctx, tag, asset(goos, goarch), dest)
}

// Use makes tag the helper in use and drops every other installed version
// except the one it replaces, which stays for a way back.
func (m *Manager) Use(tag string) {
	var old string
	m.Update(func(s *State) {
		old = s.Version
		s.Version, s.Pending = tag, ""
	})
	entries, _ := os.ReadDir(filepath.Join(m.Dir, "bin"))
	for _, e := range entries {
		if n := e.Name(); n != tag && n != old {
			_ = os.RemoveAll(filepath.Join(m.Dir, "bin", n))
		}
	}
}

// Activate switches to the version downloaded by an earlier run. It must run
// before the server starts.
func (m *Manager) Activate() {
	st := m.State()
	if st.Pending != "" && fileExists(m.BinaryPath(st.Pending)) {
		m.Use(st.Pending)
	}
}

// Outcome says what a look for a new release did.
type Outcome struct {
	Downloaded string // a newer release was downloaded and waits for a restart
	Major      string // a release with a new major version exists and was not installed
}

// Refresh looks for a newer release, at most every CheckEvery, and downloads
// it when it keeps the major version of the one in use: a new major version
// may change the config format, so it is only reported. With nothing
// installed yet it installs the newest release and uses it at once.
func (m *Manager) Refresh(ctx context.Context, now time.Time) (Outcome, error) {
	st := m.State()
	if st.Version != "" && st.Latest != "" && now.Sub(st.CheckedAt) < CheckEvery && !st.CheckedAt.After(now) {
		return m.outcome(st.Latest, st), nil
	}
	tag, err := m.Release.Latest(ctx)
	if err != nil {
		return Outcome{}, err
	}
	st = m.Update(func(s *State) { s.Latest, s.CheckedAt = tag, now })
	if st.Version == "" {
		if err := m.Install(ctx, tag); err != nil {
			return Outcome{}, err
		}
		m.Use(tag)
		return Outcome{}, nil
	}
	out := m.outcome(tag, st)
	if out.Downloaded != "" {
		if err := m.Install(ctx, tag); err != nil {
			return Outcome{}, err
		}
		m.Update(func(s *State) { s.Pending = tag })
	}
	return out, nil
}

// outcome compares the newest tag with the one in use.
func (m *Manager) outcome(latest string, st State) Outcome {
	if !update.Newer(st.Version, latest) || latest == st.Pending && st.Pending != "" {
		return Outcome{}
	}
	if major(latest) != major(st.Version) {
		return Outcome{Major: latest}
	}
	return Outcome{Downloaded: latest}
}

// major is the first number of a tag like v8.0.33.
func major(tag string) int {
	n, _ := strconv.Atoi(strings.SplitN(strings.TrimPrefix(tag, "v"), ".", 2)[0])
	return n
}

// Ensure makes sure a helper is installed and in use; it downloads the newest
// release when there is none. report gets a line for each step.
func (m *Manager) Ensure(ctx context.Context, report func(string)) error {
	if m.Installed() {
		return nil
	}
	if report != nil {
		report("looking for the newest cliproxyapi")
	}
	tag, err := m.Release.Latest(ctx)
	if err != nil {
		return fmt.Errorf("cannot reach GitHub: %w", err)
	}
	if report != nil {
		report("downloading cliproxyapi " + tag)
	}
	if err := m.Install(ctx, tag); err != nil {
		return err
	}
	m.Use(tag)
	m.Update(func(s *State) { s.Latest, s.CheckedAt = tag, time.Now() })
	return nil
}
