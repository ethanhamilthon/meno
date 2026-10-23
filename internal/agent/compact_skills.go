package agent

import (
	"sort"
	"strings"
)

// activatedSkills returns the skill files that the summarized turns (or the
// previous summary) show as read. Skills are activated by reading their file,
// so compaction would otherwise replace the full instructions with a summary.
func activatedSkills(paths []string, prevSummary string, head []Turn) []string {
	var out []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		hit := strings.Contains(prevSummary, p)
		for i := 0; !hit && i < len(head); i++ {
			hit = strings.Contains(head[i].Code, p)
		}
		if hit {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

const activatedHeading = "## Activated skills"

// withActivatedSkills appends a deterministic reminder to the summary telling
// the model to re-read the skill files it had activated.
func withActivatedSkills(summary string, skills []string) string {
	if len(skills) == 0 {
		return summary
	}
	if i := strings.Index(summary, activatedHeading); i >= 0 { // the model copied the old reminder
		summary = summary[:i]
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(summary))
	b.WriteString("\n\n" + activatedHeading + "\nThe full instructions of these skills were summarized away. Before continuing work they cover, read the file again with cat:\n")
	for _, p := range skills {
		b.WriteString("- " + p + "\n")
	}
	return strings.TrimSpace(b.String())
}
