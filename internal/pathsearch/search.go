// Package pathsearch finds files and folders for the @ picker: fuzzy, bounded,
// cancellable, and deliberately unfiltered — .git, dotfiles, dependencies and
// git-ignored files are all candidates.
package pathsearch

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Entry is one candidate. Path is absolute and keeps symlinks as they are
// (never the resolved target); Rel is relative to the search base.
type Entry struct {
	Path    string
	Rel     string
	Dir     bool
	Symlink bool
	Score   int
}

// Limits keeping a search bounded on huge trees.
const (
	MaxVisited = 40000
	MaxDepth   = 14
)

// Resolve splits a query into the directory to search and the fuzzy part. A
// query with a slash navigates: "../x" is relative to cwd, "~/x" to home and
// "/x" is absolute. dirPrefix is the typed directory text ("" without a slash).
func Resolve(cwd, home, query string) (base, fuzzy, dirPrefix string) {
	i := strings.LastIndex(query, "/")
	if i < 0 {
		return cwd, query, ""
	}
	dirPrefix, fuzzy = query[:i+1], query[i+1:]
	d := dirPrefix
	switch {
	case d == "~/" || strings.HasPrefix(d, "~/"):
		d = filepath.Join(home, d[2:])
	case !filepath.IsAbs(d):
		d = filepath.Join(cwd, d)
	}
	return filepath.Clean(d), fuzzy, dirPrefix
}

// Search walks base breadth-first (shallow entries first, so the visit cap cuts
// the deepest ones) and returns the best limit entries for fuzzy, best first,
// ties broken by path so the order is deterministic. An empty fuzzy part lists
// the children of base. Symlinked directories are listed but not entered.
func Search(ctx context.Context, base, fuzzy string, limit int) []Entry {
	if limit <= 0 {
		limit = 50
	}
	type item struct {
		dir   string
		depth int
	}
	queue := []item{{base, 0}}
	var out []Entry
	visited := 0
	for len(queue) > 0 && visited < MaxVisited {
		if ctx.Err() != nil {
			return nil
		}
		cur := queue[0]
		queue = queue[1:]
		ents, err := os.ReadDir(cur.dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			visited++
			full := filepath.Join(cur.dir, e.Name())
			rel, _ := filepath.Rel(base, full)
			isLink := e.Type()&os.ModeSymlink != 0
			isDir := e.IsDir()
			if isLink {
				if st, err := os.Stat(full); err == nil && st.IsDir() {
					isDir = true
				}
			}
			if fuzzy == "" && cur.depth > 0 {
				continue
			}
			if score, ok := Score(fuzzy, rel, e.Name()); ok {
				out = append(out, Entry{Path: full, Rel: filepath.ToSlash(rel), Dir: isDir, Symlink: isLink, Score: score})
			}
			if e.IsDir() && !isLink && cur.depth+1 < MaxDepth && fuzzy != "" {
				queue = append(queue, item{full, cur.depth + 1})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Rel < out[j].Rel
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Score ranks rel (a path) and name (its last element) against a fuzzy query:
// exact name, name prefix, name substring and name subsequence beat a
// subsequence over the whole path; shorter and shallower paths win ties. ok is
// false when the query is not a subsequence of the path at all.
func Score(query, rel, name string) (int, bool) {
	if query == "" {
		return 1000 - len(rel), true
	}
	q, n, p := strings.ToLower(query), strings.ToLower(name), strings.ToLower(rel)
	depth := strings.Count(rel, "/")
	switch {
	case n == q:
		return 10000 - depth, true
	case strings.HasPrefix(n, q):
		return 8000 - len(n) - depth*5, true
	case strings.Contains(n, q):
		return 6000 - len(n) - depth*5, true
	}
	if subsequence(q, n) {
		return 4000 - len(n) - depth*5, true
	}
	if strings.Contains(p, q) {
		return 3000 - len(p), true
	}
	if subsequence(q, p) {
		return 1000 - len(p), true
	}
	return 0, false
}

func subsequence(q, s string) bool {
	qi := []rune(q)
	k := 0
	for _, r := range s {
		if k < len(qi) && unicode.ToLower(r) == qi[k] {
			k++
		}
	}
	return k == len(qi)
}
