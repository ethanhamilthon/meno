package prompt

import (
	"strings"

	"meno/internal/jsvm"
)

// Everything about writing js lives in the description of the js tool, not in
// the system prompt: providers show the model the tool definition next to the
// system prompt, and a model reads the rules of a tool where it finds the tool.

const toolIntro = `Run JavaScript and get the value of the last expression. It is your only tool.`

const toolRuntime = `Runtime:
- The runtime lives for your current answer. Variables, functions and classes you declare at the top level of a call are there in the next calls. A new message from the user starts a clean runtime, so keep lasting data in files. Code with a top-level return or await runs inside a function, and what it declares stays local.
- The runtime is synchronous. Do not use async, await, Promise, or then. The call returns the value of the last expression. Console output and the result share 64 KB. Each call has a timeout.`

const toolGlobals = `Globals:
- fs: read, inspect, search, patch, edit, write, glob, exists. To change a file, read it with fs.inspect and pass its sha256 to fs.patch as expectedHash.
- sh(cmd) returns {stdout, stderr, code, timedOut}. view(path) shows a PNG or JPEG after the call.
- websearch(query, {count?}) searches the web and returns {query, backend, answer?, results: [{title, url, snippet}]}. Use it for fresh facts. For another HTTP API use sh(curl ...): there is no fetch, URL or env.
- console.log.`

const (
	jsGlobalsMain  = "\n- session, askUser, tasks, agents, goal, mcp, mcpTools, mcpResults, mcpServers."
	jsGlobalsChild = "\n- session, agents, mcp, mcpTools, mcpResults, mcpServers."
	jsHelpLine     = "\n- help() lists the topics (one for each of these globals, plus js and fs). help(\"name\") returns the full API of one topic. Read a topic before you use it for the first time."
)

// toolWork covers background tasks and subagents (the main agent only).
const toolWork = `Background tasks and subagents:
- tasks.start(command, opts) returns {id}. Use tasks.wait, tasks.logs, tasks.status, and tasks.cancel with the id. Tasks stop when meno exits. See help("tasks").
- agents.spawn({task}) returns {id}. A subagent cannot see this conversation, so put all needed context in task. You are responsible for the result. Check its work. See help("agents").`

// toolMCP is the fixed part about MCP; the servers are listed in the system prompt.
const toolMCP = `MCP:
- Call a tool as mcp.<server>.<tool>({...}). The call returns {text, structuredContent, isError, resultId, truncated}. Use mcpTools("server") for the input schemas. Change a server with mcpServers only when the user asks. See help("mcp") and help("mcpServers").`

// ToolDescription is the description of the js tool for the main agent or, with
// subagent, for a child (no askUser, tasks or agents.spawn).
func ToolDescription(subagent bool) string {
	globals := toolGlobals + jsGlobalsMain
	if subagent {
		globals = toolGlobals + jsGlobalsChild
	}
	parts := []string{toolIntro, toolRuntime, globals + jsHelpLine}
	if !subagent {
		parts = append(parts, toolWork)
	}
	parts = append(parts, toolMCP)
	return strings.Join(parts, "\n\n")
}

// ToolDescriptionFor is the description of the js tool when a mode keeps only
// the globals in only (nil keeps them all). The description then names what is
// left and says what is gone, so the model does not reach for it.
func ToolDescriptionFor(subagent bool, only []string) string {
	base := ToolDescription(subagent)
	if only == nil {
		return base
	}
	keep := map[string]bool{}
	for _, n := range only {
		keep[n] = true
	}
	var gone []string
	for _, n := range jsvm.ToolGlobals {
		if !keep[n] {
			gone = append(gone, n)
		}
	}
	left := []string{}
	for _, n := range only {
		left = append(left, n)
	}
	return base + "\n\nThis request runs in a restricted mode. These globals are not available and fail with ReferenceError: " +
		strings.Join(gone, ", ") + ". Only these remain: " + strings.Join(left, ", ") + "."
}
