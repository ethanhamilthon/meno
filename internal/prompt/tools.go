package prompt

import "strings"

// guidelines contains only the js tool reminder and the main agent ask rule.
const (
	baseGuidelines = "- Your only tool is js. Its description lists the globals you can use."
	askGuideline   = "- Ask: use askUser({questions}). It shows all questions in one form. See help(\"askUser\")."
)

// subagentPrompt tells a child who it is.
const subagentPrompt = `A parent agent started you for the task in your first message. You cannot ask the user, you have no background tasks, and you cannot start agents. Use agents.report(text) for a short progress note and agents.block(reason) when you cannot continue without something. Your final answer, when you stop calling tools, is your result for the parent: state what you did, what you found, and what is unverified. Follow the instructions in the task about what you may change. See help("agents").`

// guidelines is the Guidelines section. A subagent does not get the ask rule:
// it cannot ask the user.
func guidelines(subagent bool) string {
	l := []string{baseGuidelines}
	if !subagent {
		l = append(l, askGuideline)
	}
	return element("guidelines", strings.Join(l, "\n"))
}

// subagentRole is the element that tells a child who it is.
func subagentRole() string { return element("subagent-role", subagentPrompt) }
