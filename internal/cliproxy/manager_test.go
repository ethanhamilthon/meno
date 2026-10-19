package cliproxy

import (
	"os"
	"strings"
	"testing"
)

func TestStateGetsKeysAndAPortOnce(t *testing.T) {
	dir := t.TempDir()
	a := New(dir).State()
	if !strings.HasPrefix(a.ClientKey, "mc-") || !strings.HasPrefix(a.ManagementKey, "mm-") || a.Port == 0 || a.ClientKey == a.ManagementKey {
		t.Fatalf("%+v", a)
	}
	if b := New(dir).State(); b != a {
		t.Fatalf("the state must persist: %+v vs %+v", b, a)
	}
	info, err := os.Stat(New(dir).statePath())
	if err != nil || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("the state file holds keys and must be private: %v %v", info, err)
	}
}

func TestOldStateDiscoversAccountCountOnce(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if err := os.MkdirAll(m.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.statePath(), []byte(`{"has_accounts":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	st := New(dir).State()
	if st.Accounts != 1 || !st.AccountsKnown || !st.HasAccounts {
		t.Fatalf("legacy state: %+v", st)
	}
	if again := New(dir).State(); again.Accounts != 1 {
		t.Fatalf("account count was not persisted: %+v", again)
	}
}

func TestWriteConfigListensOnLoopbackOnly(t *testing.T) {
	m := New(t.TempDir())
	if err := m.WriteConfig(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(m.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	st := m.State()
	for _, want := range []string{"config-version: 8", `host: "127.0.0.1"`, "port: " + itoa(st.Port), "allow-remote: false", "disable-control-panel: true", st.ManagementKey, st.ClientKey, m.AuthDir()} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config lacks %q:\n%s", want, cfg)
		}
	}
	info, _ := os.Stat(m.ConfigPath())
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatal("the config holds keys and must be private")
	}
}

func TestCommandQuotesPaths(t *testing.T) {
	m := New(t.TempDir() + "/it's here")
	m.Update(func(s *State) { s.Version = "v8.0.1" })
	cmd := m.Command()
	if !strings.Contains(cmd, `'\''`) || !strings.Contains(cmd, " -config ") {
		t.Fatal(cmd)
	}
}

func TestYAMLQuote(t *testing.T) {
	if got := yamlQuote("a\"b\\c\nd"); got != `"a\"b\\c\nd"` {
		t.Fatal(got)
	}
}

func TestMajor(t *testing.T) {
	if major("v8.0.33") != 8 || major("10.1.2") != 10 || major("junk") != 0 {
		t.Fatal("major")
	}
}
