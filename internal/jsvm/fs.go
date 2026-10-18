package jsvm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FS struct {
	Root string
}

// path resolves relative names against the working directory and "~" against
// the home directory; absolute paths and symlinks keep their normal filesystem
// semantics. There is deliberately no sandbox: the agent may touch any file.
func (f FS) path(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(f.Root, p)
	}
	return filepath.Abs(p)
}

func (f FS) Read(p string) (string, error) {
	full, err := f.path(p)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(full)
	return string(b), err
}

func (f FS) Write(p, content string, overwrite ...bool) error {
	full, err := f.path(p)
	if err != nil {
		return err
	}
	defer lockPath(full)()
	allowOverwrite := len(overwrite) > 0 && overwrite[0]
	if !allowOverwrite {
		if _, err := os.Stat(full); err == nil {
			return fmt.Errorf("fs.write: %s already exists (use fs.patch or fs.edit to change it, or pass {overwrite: true})", p)
		}
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

func (f FS) Edit(p, oldS, newS string) (int, error) {
	full, err := f.path(p)
	if err != nil {
		return 0, err
	}
	if oldS == "" {
		return 0, fmt.Errorf("fs.edit: old string is empty")
	}
	defer lockPath(full)()
	b, err := os.ReadFile(full)
	if err != nil {
		return 0, err
	}
	src := string(b)
	n := strings.Count(src, oldS)
	if n != 1 {
		return n, fmt.Errorf("fs.edit: %s in %s", matchProblem(src, oldS, n), p)
	}
	_, _, err = writeAtomic(full, b, []byte(strings.Replace(src, oldS, newS, 1)), 1)
	return 1, err
}

func (f FS) Exists(p string) bool {
	full, err := f.path(p)
	if err != nil {
		return false
	}
	_, err = os.Stat(full)
	return err == nil
}

// Glob matches pattern against files below its longest wildcard-free directory
// prefix. Relative patterns yield relative names; absolute and ../ patterns
// yield names with the same prefix, so results can be fed back into fs.read.
func (f FS) Glob(pattern string) ([]string, error) {
	pattern = filepath.ToSlash(pattern)
	segs := strings.Split(pattern, "/")
	i := 0
	for i < len(segs)-1 && !strings.ContainsAny(segs[i], "*?") {
		i++
	}
	prefix, rest := strings.Join(segs[:i], "/"), strings.Join(segs[i:], "/")
	if prefix == "" && strings.HasPrefix(pattern, "/") {
		prefix = "/"
	}
	dir := prefix
	if dir == "" {
		dir = "."
	}
	base, err := f.path(dir)
	if err != nil {
		return nil, err
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return nil, err
	}
	var out []string
	err = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != base && (d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(base, path)
		if MatchGlob(rest, rel) {
			name := filepath.ToSlash(rel)
			switch prefix {
			case "":
			case "/":
				name = "/" + name
			default:
				name = prefix + "/" + name
			}
			out = append(out, name)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}
