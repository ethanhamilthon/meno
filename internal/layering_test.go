package internal_test

import (
	"bufio"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestLayeringImports keeps the migration target explicit. Every currently
// forbidden dependency is listed in the allow file; remove each entry as the
// TUI moves to the client boundary.
func TestLayeringImports(t *testing.T) {
	root := "."
	allowed, err := readAllowed(filepath.Join(root, "layering_allow.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"app", "ui"} {
		found, err := forbiddenImports(filepath.Join(root, pkg))
		if err != nil {
			t.Fatal(err)
		}
		for imp := range found {
			key := pkg + " " + imp
			if !allowed[key] {
				t.Errorf("%s imports forbidden %s; move it behind the daemon client", pkg, imp)
			}
		}
		for key := range allowed {
			parts := strings.SplitN(key, " ", 2)
			if parts[0] == pkg && !found[parts[1]] {
				t.Errorf("remove stale layering allow-list entry %q", key)
			}
		}
	}
}

func forbiddenImports(dir string) (map[string]bool, error) {
	bad := map[string]bool{}
	forbidden := map[string]bool{"agent": true, "llm": true, "db": true, "mcp": true, "proc": true, "cliproxy": true, "config": true}
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return nil, err
		}
		for _, spec := range f.Imports {
			imp := strings.Trim(spec.Path.Value, "\"")
			if !strings.HasPrefix(imp, "meno/internal/") {
				continue
			}
			name := strings.TrimPrefix(imp, "meno/internal/")
			name = strings.SplitN(name, "/", 2)[0]
			if forbidden[name] {
				bad[imp] = true
			}
		}
	}
	return bad, nil
}

func readAllowed(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	allowed := map[string]bool{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		allowed[line] = true
	}
	if err = s.Err(); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(allowed))
	for k := range allowed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) != len(allowed) {
		return nil, os.ErrInvalid
	}
	return allowed, nil
}
