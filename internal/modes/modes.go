// Package modes defines the modes a message can start with: #plan, #build,
// #goal and the ones the user writes. A mode is more than a prompt: it can make
// the file system read-only, keep the agent working until a goal is met, and
// hand over to the next mode.
package modes

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"meno/internal/config"
)

// DefaultRounds is how many times a goal mode asks the agent to go on when it
// answers without having called goal.done or goal.blocked.
const DefaultRounds = 25

// Mode is one mode.
type Mode struct {
	Name        string
	Description string
	// ReadOnly makes the run unable to change anything: fs writes are refused,
	// sh runs in a read-only sandbox, tasks and subagents are off.
	ReadOnly bool
	// Goal keeps the run going until the agent calls goal.done or goal.blocked,
	// for at most Rounds nudges.
	Goal   bool
	Rounds int
	// Effort replaces the reasoning effort for the run when the model has it.
	Effort string
	// Next names the mode the user is offered when a run in this mode ends.
	Next string
	// Default is the message used when the mode is written without a task.
	Default string
	// Text is what the model is told.
	Text    string
	Builtin bool
	Path    string // the file of a mode the user wrote
}

// Builtins are the modes every meno has.
func Builtins() []Mode {
	return []Mode{
		{Name: "plan", Description: "research and write a plan; nothing is changed", ReadOnly: true, Next: "build", Builtin: true, Text: planText},
		{Name: "build", Description: "carry the task out and check it", Default: "Carry out the plan above.", Builtin: true, Text: buildText},
		{Name: "goal", Description: "work until the goal is met", Goal: true, Rounds: DefaultRounds, Builtin: true, Text: goalText},
	}
}

const planText = `Mode: plan.
Research the task and write a plan. You cannot change anything in this mode: fs.write, fs.patch and fs.edit are refused, sh runs in a read-only sandbox, and tasks.start and agents.spawn are off. Read what you need, then give a concrete plan: the steps in order, the files each step changes, how each step is checked, and the risks or open questions. Use askUser only when a choice blocks the plan. End with the plan; the user starts the work with #build.`

const buildText = `Mode: build.
Carry the task out. Make the changes, run the checks of the project (build, tests, format) and fix what fails before you answer. If a plan came before, follow it and say where you left it. Finish with what changed, what you ran, and what you could not verify.`

const goalText = `Mode: goal.
The message is a goal, not one step. Work on it until it is met, and do not stop between steps to ask whether to go on. Keep the list of what is left in a variable; the runtime keeps it for your whole answer. When the goal is met and you have checked it yourself, call goal.done(summary) with what you did and how you checked it. If you cannot go on without something only the user can give, call goal.blocked(reason). If you answer without calling either, you will be asked to go on.`

// Load returns the builtin modes followed by the user's, which are the .md
// files of the given directories (later directories win). A file named like a
// builtin replaces it. The result is sorted by name.
func Load(dirs ...string) []Mode {
	byName := map[string]Mode{}
	for _, m := range Builtins() {
		byName[m.Name] = m
	}
	for _, dir := range dirs {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") || strings.EqualFold(e.Name(), "README.md") {
				continue
			}
			if m, ok := readMode(filepath.Join(dir, e.Name())); ok {
				byName[m.Name] = m
			}
		}
	}
	out := make([]Mode, 0, len(byName))
	for _, m := range byName {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func readMode(path string) (Mode, bool) {
	b, err := os.ReadFile(path)
	if err != nil || len(b) > 64<<10 {
		return Mode{}, false
	}
	meta, body := config.ParseFrontmatter(string(b))
	name := config.SkillName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if v := config.SkillName(meta["name"]); v != "" {
		name = v
	}
	body = strings.TrimSpace(body)
	if name == "" || body == "" {
		return Mode{}, false
	}
	m := Mode{
		Name: name, Description: strings.Join(strings.Fields(meta["description"]), " "), Path: path,
		ReadOnly: truthy(meta["readonly"]), Goal: truthy(meta["goal"]), Effort: meta["effort"],
		Next: config.SkillName(meta["next"]), Default: meta["default"], Text: "Mode: " + name + ".\n" + body,
	}
	if m.Goal {
		m.Rounds = DefaultRounds
		if n, err := strconv.Atoi(meta["rounds"]); err == nil && n > 0 {
			m.Rounds = min(n, 200)
		}
	}
	return m, true
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1", "on":
		return true
	}
	return false
}

// Find returns the mode called name.
func Find(list []Mode, name string) (Mode, bool) {
	for _, m := range list {
		if m.Name == name {
			return m, true
		}
	}
	return Mode{}, false
}

// Parse reads a leading #name of text. When name is a known mode it returns the
// mode and the rest of the text with the tag taken off.
func Parse(text string, list []Mode) (m Mode, rest string, ok bool) {
	t := strings.TrimLeftFunc(text, unicode.IsSpace)
	if !strings.HasPrefix(t, "#") {
		return Mode{}, text, false
	}
	end := 1
	for end < len(t) && isNameByte(t[end]) {
		end++
	}
	if end < len(t) && !unicode.IsSpace(rune(t[end])) {
		return Mode{}, text, false // "#plan-b" or "#plan." is not a tag
	}
	m, found := Find(list, strings.ToLower(t[1:end]))
	if !found {
		return Mode{}, text, false
	}
	return m, strings.TrimSpace(t[end:]), true
}

func isNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-' || b == '_'
}

// Block is the text put in front of the message for the model.
func Block(m Mode) string { return strings.TrimSpace(m.Text) }
