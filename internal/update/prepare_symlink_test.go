package update

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPrepareResolvesSymlinkAndRejectsCorruptArchive(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			arc := archive(t, "new")
			if corrupt {
				arc = []byte("not a gzip archive")
			}
			name := "meno_v1_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
			sum := sha256.Sum256(arc)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/SHA256SUMS" {
					fmt.Fprintf(w, "%x  %s\n", sum, name)
					return
				}
				w.Write(arc)
			}))
			defer srv.Close()
			dir := t.TempDir()
			dest := filepath.Join(dir, "meno-real")
			link := filepath.Join(dir, "meno")
			os.WriteFile(dest, []byte("old"), 0755)
			if err := os.Symlink(dest, link); err != nil {
				t.Fatal(err)
			}
			c := Client{Base: srv.URL}
			p, err := c.Prepare(context.Background(), "v1", link)
			if corrupt {
				if err == nil {
					p.Close()
					t.Fatal("prepared corrupt archive")
				}
				if b, _ := os.ReadFile(dest); string(b) != "old" {
					t.Fatal("corruption changed executable")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				defer p.Close()
				resolved, _ := filepath.EvalSymlinks(dest)
				if p.dest != resolved {
					t.Fatalf("symlink not resolved: %s", p.dest)
				}
				if err := p.Commit(context.Background()); err != nil {
					t.Fatal(err)
				}
				if info, _ := os.Lstat(link); info.Mode()&os.ModeSymlink == 0 {
					t.Fatal("symlink replaced")
				}
				if b, _ := os.ReadFile(dest); string(b) != "new" {
					t.Fatal("target not replaced")
				}
			}
			if left, _ := filepath.Glob(filepath.Join(dir, ".meno-update-*")); len(left) != 0 {
				t.Fatalf("staging leaked: %v", left)
			}
		})
	}
}
