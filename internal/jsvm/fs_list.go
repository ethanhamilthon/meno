package jsvm

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// listFiles returns the files below root as sorted slash-separated relative
// paths. Inside a git work tree and without includeIgnored it asks git, so
// .gitignore rules apply exactly; otherwise it walks, skipping .git and
// node_modules unless includeIgnored is set.
func listFiles(root string, includeIgnored bool) ([]string, error) {
	if !includeIgnored {
		cmd := exec.Command("git", "-C", root, "ls-files", "-co", "--exclude-standard", "-z", "--", ".")
		if out, err := cmd.Output(); err == nil {
			var files []string
			for _, f := range strings.Split(string(out), "\x00") {
				if f == "" {
					continue
				}
				if st, err := os.Lstat(filepath.Join(root, f)); err == nil && st.Mode().IsRegular() {
					files = append(files, f)
				}
			}
			sort.Strings(files)
			return files, nil
		}
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && !includeIgnored && (d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			rel, _ := filepath.Rel(root, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}
