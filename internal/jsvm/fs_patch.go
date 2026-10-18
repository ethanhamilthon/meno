package jsvm

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Replacement is one exact replacement for Patch.
type Replacement struct{ Old, New string }

// Patch applies replacements to the original contents of a file. It requires
// the hash the caller saw, every old text must occur exactly once, they may not
// overlap, and nothing is written unless all of them apply. The write is an
// atomic replace that keeps the file's permissions.
func (f FS) Patch(p string, edits []Replacement, expected string) (string, int, error) {
	full, err := f.path(p)
	if err != nil {
		return "", 0, err
	}
	defer lockPath(full)()
	if expected == "" {
		return "", 0, errors.New("fs.patch: expectedHash is required (get it from fs.inspect)")
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", 0, err
	}
	if got := hashOf(b); got != expected {
		return "", 0, fmt.Errorf("fs.patch: %s changed since it was inspected (hash is now %s); inspect it again", p, got)
	}
	src := string(b)
	type span struct{ at, end, idx int }
	var spans []span
	for i, e := range edits {
		if e.Old == "" {
			return "", 0, fmt.Errorf("fs.patch: edit %d has an empty oldText", i)
		}
		n := strings.Count(src, e.Old)
		if n != 1 {
			return "", 0, fmt.Errorf("fs.patch: edit %d: %s", i, matchProblem(src, e.Old, n))
		}
		at := strings.Index(src, e.Old)
		spans = append(spans, span{at, at + len(e.Old), i})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].at < spans[j].at })
	for i := 1; i < len(spans); i++ {
		if spans[i].at < spans[i-1].end {
			return "", 0, fmt.Errorf("fs.patch: edits %d and %d overlap", spans[i-1].idx, spans[i].idx)
		}
	}
	var out strings.Builder
	last := 0
	for _, s := range spans {
		out.WriteString(src[last:s.at])
		out.WriteString(edits[s.idx].New)
		last = s.end
	}
	out.WriteString(src[last:])
	return writeAtomic(full, b, []byte(out.String()), len(edits))
}

// writeAtomic replaces the file with next, after checking once more that it
// still holds old.
func writeAtomic(full string, old, next []byte, n int) (string, int, error) {
	if r, err := filepath.EvalSymlinks(full); err == nil {
		full = r // replace the target, not the link
	}
	st, err := os.Stat(full)
	if err != nil {
		return "", 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(full), ".meno-patch-*")
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(next); err != nil {
		tmp.Close()
		return "", 0, err
	}
	if err := tmp.Chmod(st.Mode().Perm()); err != nil {
		tmp.Close()
		return "", 0, err
	}
	if err := tmp.Close(); err != nil {
		return "", 0, err
	}
	if cur, err := os.ReadFile(full); err != nil || !bytes.Equal(cur, old) {
		return "", 0, errors.New("fs.patch: the file changed while patching; nothing was written")
	}
	if err := os.Rename(tmp.Name(), full); err != nil {
		return "", 0, err
	}
	return hashOf(next), n, nil
}

// matchProblem explains why old is not exactly one match: how many there are
// and where, or the nearest candidate line when there is none.
func matchProblem(src, old string, n int) string {
	if n > 1 {
		var where []string
		for off, k := 0, 0; k < 10; k++ {
			i := strings.Index(src[off:], old)
			if i < 0 {
				break
			}
			where = append(where, fmt.Sprintf("line %d", 1+strings.Count(src[:off+i], "\n")))
			off += i + len(old)
		}
		return fmt.Sprintf("old text matches %d times (%s); add surrounding context to make it unique", n, strings.Join(where, ", "))
	}
	first := strings.TrimSpace(strings.SplitN(strings.TrimSpace(old), "\n", 2)[0])
	if first != "" {
		for i, line := range strings.Split(src, "\n") {
			if strings.Contains(line, first) {
				return fmt.Sprintf("old text not found; its first line appears at line %d: %q (the following lines differ)", i+1, clip(line, 120))
			}
		}
		for i, line := range strings.Split(src, "\n") {
			if strings.EqualFold(strings.Join(strings.Fields(line), " "), strings.Join(strings.Fields(first), " ")) {
				return fmt.Sprintf("old text not found; line %d matches apart from whitespace or case: %q", i+1, clip(line, 120))
			}
		}
	}
	return "old text not found"
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
