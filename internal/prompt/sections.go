package prompt

import (
	"fmt"
	"sort"
	"strings"
)

// skillsIntro opens the skills element.
const skillsIntro = "A skill is an instruction file for one type of work. When the task matches a skill, read its file with fs.read before you start. Obey the file. If the user writes /name, read that skill first."

func skillsSection(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	skills = append([]Skill(nil), skills...)
	sort.SliceStable(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	var l []string
	for _, s := range skills {
		line := "- " + s.Name
		if d := short(s.Desc, 200); d != "" {
			line += " — " + d
		}
		l = append(l, line+" ("+s.Path+")")
	}
	return element("skills", skillsIntro+"\n"+strings.Join(l, "\n"))
}

// instructionsSection wraps one AGENTS.md in the element called name.
func instructionsSection(name string, in Instructions) string {
	if strings.TrimSpace(in.Text) == "" {
		return ""
	}
	return element(name, "From "+in.Path+". Follow these unless the user's message says otherwise.\n\n"+in.Text)
}

// mcpToolsSection lists the running servers with the names of their tools, up
// to maxListedTools names in all; mcpTools() gives the rest and the schemas.
func mcpToolsSection(tools []Tool) string {
	if len(tools) == 0 {
		return ""
	}
	tools = append([]Tool(nil), tools...)
	sort.SliceStable(tools, func(i, j int) bool {
		if tools[i].Server != tools[j].Server {
			return tools[i].Server < tools[j].Server
		}
		return tools[i].Name < tools[j].Name
	})
	var servers []string
	byServer := map[string][]string{}
	for _, t := range tools {
		if _, ok := byServer[t.Server]; !ok {
			servers = append(servers, t.Server)
		}
		byServer[t.Server] = append(byServer[t.Server], ident(t.Name))
	}
	listed := 0
	var l []string
	for _, s := range servers {
		names := byServer[s]
		noun := "tools"
		if len(names) == 1 {
			noun = "tool"
		}
		line := fmt.Sprintf("- %s (%d %s)", ident(s), len(names), noun)
		if listed >= maxListedTools {
			l = append(l, line)
			continue
		}
		room := min(len(names), maxListedTools-listed)
		listed += room
		line += ": " + strings.Join(names[:room], ", ")
		if room < len(names) {
			line += fmt.Sprintf(", … and %d more", len(names)-room)
		}
		l = append(l, line)
	}
	return element("mcp-servers", strings.Join(l, "\n"))
}

func environmentSection(in Input) string {
	lines := []string{"- Working directory: " + in.Cwd}
	switch {
	case in.OS != "" && in.Shell != "":
		lines = append(lines, "- OS: "+in.OS+". Shell: "+in.Shell+".")
	case in.OS != "":
		lines = append(lines, "- OS: "+in.OS+".")
	case in.Shell != "":
		lines = append(lines, "- Shell: "+in.Shell+".")
	}
	if !in.Now.IsZero() {
		lines = append(lines, "- Date: "+FormatDate(in.Now)+".")
	}
	if in.Model != "" {
		lines = append(lines, "- Model: "+DescribeModel(in.Provider, in.Model, in.Effort)+".")
	}
	return element("environment", strings.Join(lines, "\n"))
}
