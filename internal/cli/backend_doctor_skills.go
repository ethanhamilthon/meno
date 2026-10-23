package cli

import (
	"fmt"
	"os"

	"meno/internal/config"
)

// skillChecks reports authoring warnings of the skills visible from the
// current directory. Skills load leniently, so these are warnings, not errors.
func skillChecks(home string) []Check {
	cwd, _ := os.Getwd()
	paths := config.Paths{Home: home, Cwd: cwd}
	var out []Check
	for _, s := range config.DiscoverSkillDirs(paths.SkillDirs()) {
		for _, w := range s.Warnings {
			out = append(out, Check{Name: "skill " + s.Name, Status: "warn", Detail: oneLine(w), Fix: fmt.Sprintf("edit %s", s.Path)})
		}
	}
	return out
}
