package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPreparedDoesNotReplaceUntilCommit(t *testing.T) {
	srv := release(t, "v0.2.0", "new", "")
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "meno")
	os.WriteFile(dest, []byte("old"), 0755)
	c := Client{Base: srv.URL + "/dl"}
	p, err := c.PrepareAsset(context.Background(), "v0.2.0", Asset{Sums: "SHA256SUMS", Suffix: "_linux_amd64.tar.gz", Binary: "meno"}, dest)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if b, _ := os.ReadFile(dest); string(b) != "old" {
		t.Fatal("prepare changed executable")
	}
	info, err := os.Stat(p.path)
	if err != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("permissions %v %v", info, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Commit(ctx); err == nil {
		t.Fatal("committed cancelled update")
	}
	if err := p.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "new" {
		t.Fatal("commit did not replace executable")
	}
}

func TestPreparedFailureAndCleanup(t *testing.T) {
	srv := release(t, "v0.2.0", "new", "")
	defer srv.Close()
	for _, name := range []string{"rename", "cancel", "archive", "permissions"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "meno")
			os.WriteFile(dest, []byte("old"), 0755)
			c := Client{Base: srv.URL + "/dl"}
			a := Asset{Sums: "SHA256SUMS", Suffix: "_linux_amd64.tar.gz", Binary: "meno"}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if name == "cancel" {
				cancel()
			}
			if name == "archive" {
				a.Binary = "missing"
			}
			if name == "permissions" {
				dest = filepath.Join(dest, "child")
			}
			p, err := c.PrepareAsset(ctx, "v0.2.0", a, dest)
			if name == "rename" {
				if err != nil {
					t.Fatal(err)
				}
				os.Remove(dest)
				os.Mkdir(dest, 0755)
				if err := p.Commit(ctx); err == nil {
					t.Fatal("rename should fail")
				}
				if err := p.Close(); err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				p.Close()
				t.Fatal("prepare should fail")
			}
			if left, _ := filepath.Glob(filepath.Join(dir, ".meno-update-*")); len(left) != 0 {
				t.Fatalf("staging leaked: %v", left)
			}
		})
	}
}
