package jsvm

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// pathLocks serialize writers inside this process, so two tools patching the
// same file cannot interleave. They cannot protect against other programs:
// rename is atomic, but it is not a compare-and-swap, so the hash is checked
// again right before the rename and a conflicting external write in that tiny
// window is still possible.
var pathLocks sync.Map

func lockPath(p string) func() {
	m, _ := pathLocks.LoadOrStore(p, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Inspected is a file excerpt with the hash of the whole file.
type Inspected struct {
	Text      string
	SHA256    string
	StartLine int
	EndLine   int
}

// splitLines splits keeping the line terminators, so joining restores the text.
func splitLines(s string) []string {
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

// Inspect reads lines start..end (inclusive, one-based; 0 = file start/end).
// The hash is always that of the entire original file.
func (f FS) Inspect(p string, start, end int) (Inspected, error) {
	full, err := f.path(p)
	if err != nil {
		return Inspected{}, err
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return Inspected{}, err
	}
	lines := splitLines(string(b))
	n := len(lines)
	if start == 0 {
		start = 1
	}
	if end == 0 {
		end = n
	}
	if start < 1 || end < 1 {
		return Inspected{}, fmt.Errorf("line numbers start at 1 (got %d..%d)", start, end)
	}
	if end < start && !(n == 0 && end == 0) {
		return Inspected{}, fmt.Errorf("endLine %d is before startLine %d", end, start)
	}
	if start > n && !(n == 0 && start == 1) {
		return Inspected{}, fmt.Errorf("startLine %d is past the end of the file (%d lines)", start, n)
	}
	end = min(end, n)
	return Inspected{Text: strings.Join(lines[min(start-1, n):end], ""), SHA256: hashOf(b), StartLine: start, EndLine: end}, nil
}

// ReadRange is Inspect without the hash.
func (f FS) ReadRange(p string, start, end int) (string, error) {
	r, err := f.Inspect(p, start, end)
	return r.Text, err
}

// SearchOptions control FS.Search.
type SearchOptions struct {
	Root            string
	Glob            string
	Regex           bool
	CaseInsensitive bool
	IncludeIgnored  bool
	Limit           int
	Cursor          string
}

// Match is one hit; Column is a one-based Unicode scalar position.
type Match struct {
	Path   string
	Line   int
	Column int
	Text   string
}

// SearchResult is a page of matches in deterministic path/line/column order.
type SearchResult struct {
	Matches    []Match
	NextCursor string
	Truncated  bool
}

const (
	maxSearchLimit = 1000
	maxSearchFile  = 8 << 20
)

// Search finds query in text files below Root (default: the working directory).
// Without IncludeIgnored it skips what git ignores (and .git, node_modules
// outside a repository). More results than Limit set Truncated and return a
// cursor to continue from.
func (f FS) Search(query string, o SearchOptions) (SearchResult, error) {
	if query == "" {
		return SearchResult{}, errors.New("fs.search: query is empty")
	}
	limit := o.Limit
	if limit <= 0 {
		limit = 100
	}
	limit = min(limit, maxSearchLimit)
	root, err := f.path(or(o.Root, "."))
	if err != nil {
		return SearchResult{}, err
	}
	pat := query
	if !o.Regex {
		pat = regexp.QuoteMeta(query)
	}
	if o.CaseInsensitive {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return SearchResult{}, fmt.Errorf("fs.search: bad regex: %v", err)
	}
	var globRe *regexp.Regexp
	if o.Glob != "" {
		if globRe, err = GlobToRegexp(filepath.ToSlash(o.Glob)); err != nil {
			return SearchResult{}, err
		}
	}
	files, err := listFiles(root, o.IncludeIgnored)
	if err != nil {
		return SearchResult{}, err
	}
	afterPath, afterLine, afterCol, err := decodeSearchCursor(o.Cursor)
	if err != nil {
		return SearchResult{}, err
	}
	var res SearchResult
	for _, rel := range files {
		if afterPath != "" && rel < afterPath {
			continue
		}
		if globRe != nil && !globRe.MatchString(rel) && !globRe.MatchString(filepath.Base(rel)) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil || len(b) > maxSearchFile || bytes.IndexByte(b[:min(len(b), 8192)], 0) >= 0 {
			continue
		}
		for ln, line := range strings.Split(string(b), "\n") {
			ln++
			if rel == afterPath && ln < afterLine {
				continue
			}
			for _, loc := range re.FindAllStringIndex(line, -1) {
				col := utf8.RuneCountInString(line[:loc[0]]) + 1
				if rel == afterPath && (ln < afterLine || ln == afterLine && col <= afterCol) {
					continue
				}
				if len(res.Matches) == limit {
					res.Truncated = true
					last := res.Matches[len(res.Matches)-1]
					res.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s\x00%d\x00%d", last.Path, last.Line, last.Column)))
					return res, nil
				}
				res.Matches = append(res.Matches, Match{Path: rel, Line: ln, Column: col, Text: clip(strings.TrimRight(line, "\r"), 300)})
			}
		}
	}
	return res, nil
}

func or(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func decodeSearchCursor(c string) (path string, line, col int, err error) {
	if c == "" {
		return "", 0, 0, nil
	}
	raw, e := base64.RawURLEncoding.DecodeString(c)
	parts := strings.Split(string(raw), "\x00")
	if e != nil || len(parts) != 3 {
		return "", 0, 0, errors.New("fs.search: invalid cursor")
	}
	fmt.Sscan(parts[1], &line)
	fmt.Sscan(parts[2], &col)
	return parts[0], line, col, nil
}
