package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// maxCdRows bounds how many directories the /cd completion lists.
const maxCdRows = 500

// cdRows are the completions of "/cd <partial path>" as command rows
// ("/cd " + path with a trailing slash), and whether the command line is a /cd
// at all. The directory listing is cached per input and working directory, as
// the rows are asked for on every frame.
func (a *App) cdRows() ([]string, bool) {
	cmd := a.cmdQuery()
	arg, ok := strings.CutPrefix(cmd, "cd ")
	if !ok {
		return nil, false
	}
	key := a.st.Cwd + "\x00" + arg
	if a.cdKey == key {
		return a.cdCache, true
	}
	a.cdKey, a.cdCache = key, completeDirs(a.st.Cwd, arg)
	return a.cdCache, true
}

// completeDirs lists the directories that continue the partial path arg, where
// relative paths are relative to cwd. Rows keep the prefix as the user typed it.
func completeDirs(cwd, arg string) []string {
	typedDir, base := "", arg
	if i := strings.LastIndex(arg, "/"); i >= 0 {
		typedDir, base = arg[:i+1], arg[i+1:]
	}
	dir := expandHome(typedDir)
	switch {
	case typedDir == "":
		dir = cwd
	case !filepath.IsAbs(dir):
		dir = filepath.Join(cwd, dir)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		name := e.Name()
		if !strings.HasPrefix(name, base) || strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(dir, name)); err == nil {
				isDir = st.IsDir()
			}
		}
		if isDir {
			out = append(out, "/cd "+typedDir+name+"/")
		}
	}
	sort.Strings(out)
	if len(out) > maxCdRows {
		out = out[:maxCdRows]
	}
	return out
}
