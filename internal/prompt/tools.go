package prompt

import "strings"

// baseGuidelines and askGuideline are the bullets of the Guidelines section.
// The ask rule is last; a subagent does not get it.
const (
	baseGuidelines = `- Your only tool is js. Its description lists the globals you can use.
- Batch independent work in one js call. Use tasks or agents for long parallel work.
- Explore first. Implement only when you have enough information.
- Give short final answers. Do not explain details if the user did not ask.`
	askGuideline = "- Ask: use askUser({questions}). It shows all questions in one form. See help(\"askUser\")."
)

// subagentPrompt tells a child who it is.
const subagentPrompt = `## You are a subagent
A parent agent started you for the task in your first message. You cannot ask the user, you have no background tasks, and you cannot start agents. Use agents.report(text) for a short progress note and agents.block(reason) when you cannot continue without something. Your final answer, when you stop calling tools, is your result for the parent: state what you did, what you found, and what is unverified. Follow the instructions in the task about what you may change. See help("agents").`

// guidelines is the Guidelines section. A subagent does not get the ask rule:
// it cannot ask the user.
func guidelines(subagent bool) string {
	l := []string{"## Guidelines", baseGuidelines}
	if !subagent {
		l = append(l, askGuideline)
	}
	return strings.Join(l, "\n")
}
