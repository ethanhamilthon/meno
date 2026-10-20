// Package modes defines #brainstorm and #goal. A mode is a prompt, a set of tool globals the run keeps, and
// optionally a goal that keeps the agent working. Several modes combine (see
// Compose). A mode set in a message stays until another message sets it.
package modes

// DefaultRounds is how many times a goal run asks the agent to go on when it
// answers without a verdict.
const DefaultRounds = 25

// Command groups available to a bash tool.
const (
	GroupMCP    = "mcp"
	GroupTasks  = "tasks"
	GroupAsk    = "ask"
	GroupGoal   = "goal"
	GroupWeb    = "web"
	GroupView   = "view"
	GroupModels = "models"
	GroupHelp   = "help"
)

// Mode is one mode.
type Mode struct {
	Name        string
	Description string
	// Goal keeps the run going until the agent calls goal.done or goal.blocked.
	Goal   bool
	Rounds int
	// Only lists the tool globals the run keeps; nil keeps all of them.
	Only []string
	// Allow lists command groups available to bash; nil allows every group.
	Allow []string
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
	}
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

// Block is the text put in front of the message for the model.
func Block(m Mode) string { return m.Text }
