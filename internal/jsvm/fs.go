package jsvm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FS struct {
	Gate *Gate
}

func (f FS) resolve(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(f.Gate.Root, p)
}

func (f FS) Read(p string) (string, error) {
	if err := f.Gate.CheckPath("fs.read", p); err != nil {
		return "", err
	}
	b, err := os.ReadFile(f.resolve(p))
	return string(b), err
}

func (f FS) Write(p, content string) error {
	if err := f.Gate.CheckPath("fs.write", p); err != nil {
		return err
	}
	full := f.resolve(p)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

func (f FS) Edit(p, oldS, newS string) (int, error) {
	if err := f.Gate.CheckPath("fs.edit", p); err != nil {
		return 0, err
	}
	if oldS == "" {
		return 0, fmt.Errorf("fs.edit: old string is empty")
	}
	full := f.resolve(p)
	b, err := os.ReadFile(full)
	if err != nil {
		return 0, err
	}
	src := string(b)
	n := strings.Count(src, oldS)
	if n == 0 {
		return 0, fmt.Errorf("fs.edit: old string not found in %s", p)
	}
	if n > 1 {
		return n, fmt.Errorf("fs.edit: old string matches %d times in %s", n, p)
	}
	return 1, os.WriteFile(full, []byte(strings.Replace(src, oldS, newS, 1)), 0o644)
}

func (f FS) Exists(p string) bool {
	if f.Gate.CheckPath("fs.exists", p) != nil {
		return false
	}
	_, err := os.Stat(f.resolve(p))
	return err == nil
}

func (f FS) Glob(pattern string) ([]string, error) {
	if err := f.Gate.CheckPath("fs.glob", "."); err != nil {
		return nil, err
	}
	var out []string
	err := filepath.WalkDir(f.Gate.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(f.Gate.Root, path)
		if MatchGlob(pattern, rel) {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}
