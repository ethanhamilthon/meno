package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func hashBytes(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// splitFileLines preserves line terminators and omits a phantom final empty line.
func splitFileLines(b []byte) []string {
	s := string(b)
	if s == "" {
		return nil
	}
	var out []string
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i+1])
		s = s[i+1:]
	}
	return out
}

// applyEdits applies unique, non-overlapping edits without changing src.
func applyEdits(src string, edits []replacement) (string, error) {
	type span struct{ start, end, index int }
	spans := make([]span, 0, len(edits))
	for i, e := range edits {
		if e.Old == "" {
			return "", fmt.Errorf("edit %d has empty oldText", i)
		}
		n := strings.Count(src, e.Old)
		if n != 1 {
			return "", fmt.Errorf("edit %d: old text %s", i, matchIssue(src, e.Old))
		}
		at := strings.Index(src, e.Old)
		spans = append(spans, span{at, at + len(e.Old), i})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	for i := 1; i < len(spans); i++ {
		if spans[i].start < spans[i-1].end {
			return "", fmt.Errorf("edits %d and %d overlap", spans[i-1].index, spans[i].index)
		}
	}
	var b strings.Builder
	last := 0
	for _, s := range spans {
		b.WriteString(src[last:s.start])
		b.WriteString(edits[s.index].New)
		last = s.end
	}
	b.WriteString(src[last:])
	return b.String(), nil
}

// matchIssue gives line context for missing or ambiguous old text.
func matchIssue(src, old string) string {
	n := strings.Count(src, old)
	if n > 1 {
		lines := []string{}
		offset := 0
		for i := 0; i < 10; i++ {
			at := strings.Index(src[offset:], old)
			if at < 0 {
				break
			}
			lines = append(lines, fmt.Sprintf("line %d", 1+strings.Count(src[:offset+at], "\n")))
			offset += at + len(old)
		}
		return fmt.Sprintf("matches %d times (%s); add surrounding context to make it unique", n, strings.Join(lines, ", "))
	}
	oldLines := strings.Split(strings.TrimSuffix(old, "\n"), "\n")
	firstIndex := 0
	for firstIndex < len(oldLines) && strings.TrimSpace(oldLines[firstIndex]) == "" {
		firstIndex++
	}
	if firstIndex >= len(oldLines) {
		return "was not found"
	}
	first := strings.TrimSpace(oldLines[firstIndex])
	srcLines := strings.Split(src, "\n")
	bestStart, bestMatched, stop := -1, 0, 0
	fileLine, oldLine := "", ""
	for i, line := range srcLines {
		if !strings.Contains(line, first) || i < firstIndex {
			continue
		}
		start := i - firstIndex
		matched := 0
		for k := 0; k < len(oldLines); k++ {
			si := start + k
			if si >= len(srcLines) {
				if k > bestMatched || bestStart < 0 {
					bestStart, bestMatched, stop = start, k, si+1
					fileLine, oldLine = "<EOF>", oldLines[k]
				}
				break
			}
			ok := srcLines[si] == oldLines[k] || (k == 0 && strings.HasSuffix(srcLines[si], oldLines[k])) || (k == len(oldLines)-1 && strings.HasPrefix(srcLines[si], oldLines[k])) || (k == firstIndex && k == 0 && strings.Contains(srcLines[si], first) && len(oldLines) > 1)
			if !ok {
				if k > bestMatched || bestStart < 0 {
					bestStart, bestMatched, stop = start, k, si+1
					fileLine, oldLine = srcLines[si], oldLines[k]
				}
				break
			}
			matched++
		}
		if matched == len(oldLines) && bestStart < 0 {
			bestStart, bestMatched, stop = start, matched, start+1
			fileLine, oldLine = srcLines[start], oldLines[0]
		}
	}
	if bestStart >= 0 {
		if len(oldLines) == 1 || stop == bestStart+1 {
			return fmt.Sprintf("was not found; its first line appears at line %d: %q", bestStart+1, clipText(fileLine, 120))
		}
		return fmt.Sprintf("was not found; its first line appears at line %d, stopped matching at line %d (file has %q, old has %q)", bestStart+1, stop, clipText(fileLine, 120), clipText(oldLine, 120))
	}
	for i, line := range srcLines {
		if strings.EqualFold(strings.Join(strings.Fields(line), " "), strings.Join(strings.Fields(first), " ")) {
			return fmt.Sprintf("was not found; line %d matches apart from whitespace or case: %q", i+1, clipText(line, 120))
		}
	}
	return "was not found"
}
func clipText(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// atomicEdit replaces a file only if its contents still match old.
func atomicEdit(path string, old, next []byte) error {
	defer lockFile(path)()
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".meno-edit-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(next); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Chmod(st.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	cur, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(cur, old) {
		return errors.New("file changed while editing; nothing was written")
	}
	return os.Rename(tmp.Name(), path)
}
