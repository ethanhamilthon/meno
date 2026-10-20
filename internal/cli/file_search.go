package cli

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type searchMatch struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Text   string `json:"text"`
}

// searchFiles ports the deterministic, ignored-file-aware search behavior of fs.search.
func searchFiles(args []string, streams IO, jsonOut bool) (int, error) {
	values, switches, pos, err := parseFlags(args, map[string]bool{"glob": true, "limit": true}, map[string]bool{"regex": true})
	if err != nil {
		return 0, err
	}
	if len(pos) < 1 || len(pos) > 2 {
		return 0, usageError("usage: meno search PATTERN [DIR] [--regex --glob G --limit N]")
	}
	limit, err := intFlag("limit", values["limit"], 100, 1)
	if err != nil {
		return 0, err
	}
	limit = min(limit, 1000)
	pattern := pos[0]
	if pattern == "" {
		return 0, usageError("search pattern is empty")
	}
	if !switches["regex"] {
		pattern = regexp.QuoteMeta(pattern)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, fmt.Errorf("invalid search regex: %v", err)
	}
	var glob *regexp.Regexp
	if values["glob"] != "" {
		glob, err = regexp.Compile(cliGlobRegex(filepath.ToSlash(values["glob"])))
		if err != nil {
			return 0, err
		}
	}
	root := "."
	if len(pos) == 2 {
		root = pos[1]
	}
	root = cleanPath(root, streams)
	files, err := searchFileList(root)
	if err != nil {
		return 0, err
	}
	matches := make([]searchMatch, 0, limit+1)
	truncated := false
filesLoop:
	for _, rel := range files {
		if glob != nil && !glob.MatchString(filepath.ToSlash(rel)) && !glob.MatchString(filepath.Base(rel)) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil || len(b) > 8<<20 || bytes.IndexByte(b[:min(len(b), 8192)], 0) >= 0 || !utf8.Valid(b) {
			continue
		}
		for lineNo, line := range strings.Split(string(b), "\n") {
			for _, loc := range re.FindAllStringIndex(line, -1) {
				col := utf8.RuneCountInString(line[:loc[0]]) + 1
				matches = append(matches, searchMatch{filepath.ToSlash(rel), lineNo + 1, col, clipText(strings.TrimRight(line, "\r"), 300)})
				if len(matches) > limit {
					truncated = true
					break filesLoop
				}
			}
		}
	}
	if truncated {
		matches = matches[:limit]
	}
	if jsonOut {
		return 0, emit(streams, true, map[string]any{"matches": matches, "truncated": truncated}, "")
	}
	var out strings.Builder
	for _, m := range matches {
		fmt.Fprintf(&out, "%s:%d:%d:%s\n", m.Path, m.Line, m.Column, m.Text)
	}
	_, err = io.WriteString(streams.Out, out.String())
	return 0, err
}

// searchFileList uses git ignore rules when possible, then a bounded safe walk.
func searchFileList(root string) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "ls-files", "-co", "--exclude-standard", "-z", "--", ".")
	if out, err := cmd.Output(); err == nil {
		var files []string
		for _, p := range strings.Split(string(out), "\x00") {
			if p == "" {
				continue
			}
			if st, err := os.Lstat(filepath.Join(root, p)); err == nil && st.Mode().IsRegular() {
				files = append(files, p)
			}
		}
		sort.Strings(files)
		return files, nil
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && (d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			rel, _ := filepath.Rel(root, path)
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}
func cliGlobRegex(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return b.String()
}
