// Package modes defines the modes a message can name: #subagents, #brainstorm
// and #goal. A mode is a prompt, a set of tool globals the run keeps, and
// optionally a goal that keeps the agent working. Several modes combine (see
// Compose). A mode set in a message stays until another message sets it.
package modes

// DefaultRounds is how many times a goal run asks the agent to go on when it
// answers without a verdict.
const DefaultRounds = 25

// Mode is one mode.
type Mode struct {
	Name        string
	Description string
	// Goal keeps the run going until the agent calls goal.done or goal.blocked.
	Goal   bool
	Rounds int
	// Only lists the tool globals the run keeps; nil keeps all of them.
	Only []string
	// Text is what the model is told.
	Text string
}

// Control is the word that clears the modes of a session: #none.
const Control = "none"

// Catalog returns the modes meno has, sorted by name.
func Catalog() []Mode {
	return []Mode{
		{Name: "brainstorm", Description: "explore ideas, compare options and write a design spec", Text: brainstormText},
		{Name: "goal", Description: "work until the goal is met", Goal: true, Rounds: DefaultRounds, Text: goalText},
		{Name: "subagents", Description: "orchestrate: delegate all work to subagents", Only: subagentGlobals, Text: subagentsText},
	}
}

// subagentGlobals are what the orchestrator keeps: it reads and writes nothing
// itself. The pure helpers stay, they change nothing.
var subagentGlobals = []string{"agents", "goal", "askUser", "help", "hash", "path", "base64", "btoa", "atob"}

// Find returns the mode called name.
func Find(list []Mode, name string) (Mode, bool) {
	for _, m := range list {
		if m.Name == name {
			return m, true
		}
	}
	return Mode{}, false
}

// Block is the text put in front of the message for the model.
func Block(m Mode) string { return m.Text }
