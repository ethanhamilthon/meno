package modes

const brainstormText = `Mode: brainstorm.
Explore the problem before anything is built. Research the options, including the code and the web, and propose several distinct ideas. For each one give the pros, the cons and the risks. Ask the user when a choice is theirs to make. Build throwaway prototypes only in a temporary directory, never in the project, and say so. When the design is settled, write one complete spec: every decision made, why it was made, and what was rejected. The spec is not the code: it says what to build and how the parts fit, and another agent writes the code from it. Put the spec in .report/specs/ of the project.`

const goalText = `Mode: goal.
The message is a goal, not one step. Work on it until it is met, and do not stop between steps to ask whether to go on. Keep the list of what is left in a variable; the runtime keeps it for your whole answer. When the goal is met and you have checked it yourself, call goal.done(summary) with what you did and how you checked it. If you cannot go on without something only the user can give, call goal.blocked(reason). If you answer without calling either, you will be asked to go on.`
