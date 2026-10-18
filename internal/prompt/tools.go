package prompt

// jsToolPrompt documents the only tool and the globals of its runtime.
const jsToolPrompt = `## The js tool
Your only tool is js. Every call runs JavaScript in a fresh runtime: nothing you declare or change survives to the next call, so keep anything durable in state (below) or in files. The js runtime is synchronous: every host function returns its result directly. Never use async, await, Promise or then; just call the function and use the value. A promise that stays pending is an error. The value of the last expression, or of an explicit top-level return, is returned: strings as they are, objects and arrays as JSON; functions, Map, Set and cyclic objects are errors. console.log output and the returned value share one 64 KB budget per call, and every call has a timeout.
Globals:
- fs.read(path, {startLine?, endLine?}) -> string; fs.inspect(path, {startLine?, endLine?}) -> {text, sha256, startLine, endLine} (the hash covers the whole file; lines are 1-based and inclusive).
- fs.search(query, {root?, glob?, regex?, caseSensitive?, includeIgnored?, limit?, cursor?}) -> {matches: [{path, line, column, text}], nextCursor, truncated}. It skips git-ignored files unless includeIgnored is true; continue with nextCursor.
- fs.patch(path, [{oldText, newText}], {expectedHash}) -> {sha256, replacements}: exact replacements against what fs.inspect showed. Each oldText must occur exactly once, edits may not overlap, nothing is written unless all apply, and a changed file is refused. fs.edit(path, old, new) replaces one unique match; fs.write(path, text); fs.glob(pattern); fs.exists(path). Relative paths resolve against the working directory.
- sh(cmd) -> {stdout, stderr, code, timedOut}: run a shell command in the working directory.
- view(path): look at a PNG or JPEG file. The image is queued and shown to you as a message right after this call finishes; the call itself only returns a confirmation.
- fetch(url, init), env (a copy for this call), URL, URLSearchParams, console.log(...).
- state, session, sessions, askUser, mcpServers, mcp and mcpResults: described below.`

// statePrompt documents the persistent state map.
const statePrompt = `## State
state is a durable JSON map of this session. state.set(key, value), state.get(key) (a copy; undefined when missing), state.has(key), state.delete(key) -> whether it existed, state.listKeys(), state.listValues(), state.listEntries(). Keys are non-empty strings. Values must be strict JSON: null, booleans, strings, finite numbers, arrays and plain objects; anything else (undefined, functions, Date, Map, NaN, cycles) throws and names the offending path. A write is saved before set returns and survives a later error. State is not shown to you automatically: read it when you resume a task and after the context was compacted.`

// sessionPrompt documents the session and sessions globals.
const sessionPrompt = `## Session
- session.rename(name): name this session, at most 6 words, ideally 4. You must always call it: in your first js call of a session, before other work, and again when the task changes materially or when a system event says the session was forked. Never leave a session unnamed. Do not rename every step.
- session.models({provider?, refresh?}) -> {models, errors}: models of the enabled providers with what is known about them (efforts, contextWindow, images, prices; null means unknown). Some providers may fail while others succeed.
- session.compact(summary, saveLastNMessages): replace older context with your summary, keeping at least that many recent messages and the current tool batch. It takes effect after this tool batch; only one per batch. Write the summary in Markdown: goal and acceptance criteria, current and superseded decisions, done and remaining work, constraints and exclusions, changed files and Git state, what is validated and what is not, artifact and result IDs, next action and blockers. The originals stay retrievable.
- sessions.list({workdir?, after?, before?, limit?, cursor?}), sessions.search(query, same options) and sessions.read(id, {from?, limit?, includeToolOutput?, includeThinking?}) look through all local sessions: {items, nextCursor} pages (default 20, max 100) and bounded message reads that continue at next. Tool output and thinking are omitted unless asked for; omitted parts are reported with their size.`

// askPrompt documents askUser.
const askPrompt = `## Asking the user
askUser({questions: [{id, text, type, options?, recommendation?}], timeout?}) shows ALL questions at once as one form and returns {answers: [{id, selected, text}]} in the same order. type is "single", "multi" or "text"; single and multi need options [{id, label}] (at most 20), text takes none. Ids are unique non-empty strings, at most 20 questions. Every question is required and the user can always add custom text: for single, custom text is an alternative to one option (then selected is empty and text holds it); for multi, selected options and text can both be present; text is the answer to a text question. recommendation is only shown to the user, it is never selected for them. timeout is seconds (0 or missing waits as long as it takes).
If the user cancels, askUser throws AskCancelled; if time runs out, AskTimeout. Catch them if you can go on, and do not ask the same rejected form again. Use askUser only when the answer cannot be found by reading files, running commands or searching the project, and never to ask permission for ordinary actions. Group related uncertainties into one form, keep questions short, offer options when there are few.`

// todoPrompt documents the plan and acceptance criteria.
const todoPrompt = `## Plan and acceptance criteria
You keep one plan per session; the user sees it but cannot edit it. Steps and acceptance criteria are separate.
- todo.add({title, note?}), todo.update(id, {title?, note?, status?, position?}), todo.done(id, {note?}), todo.remove(id), todo.list({includeArchived?}) -> {plan, items, criteria, removed}, todo.archive({reason}). Step statuses: pending, in_progress, blocked, done, cancelled. Adding after an archive starts a new plan.
- todo.criteria.add({text}), todo.criteria.update(id, {text?, status?, evidence?, refs?, agreementRef?}), todo.criteria.list(), todo.criteria.remove(id, {reason}). Criterion statuses: not_checked, passed, failed, deferred. passed and failed need evidence (what you ran or read and what you saw; refs like result:<id>, task:<id>, msg:<id>, file:<path>). deferred needs the reason and agreementRef msg:<id> of the user message where they agreed to defer it. Removing leaves a visible tombstone.
Write the criteria when the scope is agreed. A done step does not pass a criterion. Before you finish, compare the agreed scope, what you built and the evidence. If a criterion is failed or not_checked, say so: report the work as unverified or blocked and explain what is missing, instead of claiming it is complete. Partial results are fine when described honestly.`

// tasksPrompt documents background processes (main agent only).
const tasksPrompt = `## Background tasks
tasks run shell commands in the background while you keep working: tasks.start(command, {cwd?, env?, name?, timeout?, notify?}) -> {id} returns as soon as the process exists (stdin is closed, there is no terminal; timeout is seconds of process lifetime, none by default). tasks.list({includeFinished?}), tasks.status(id), tasks.logs(id, {cursor?, limit?, stream?}) -> {records: [{seq, stream, text}], nextCursor, lost, truncated} (default: the newest 200 lines, at most 64 KB; pass nextCursor to continue; lost counts output dropped by log rotation), tasks.wait(id, {timeout?}) -> task with waitTimedOut (a timed-out wait never cancels the task), tasks.cancel(id). Statuses: starting, running, succeeded, failed, cancelled, timed_out, interrupted; exitCode is null when there was no normal exit.
When a task finishes you get an event with its id, status and exit code (not its log) unless you started it with notify: false or already saw the result through a successful tasks.wait. Tasks live only while meno is open and are cancelled when it exits. The user stopping your request cancels the tasks of that request.`

// agentsPrompt documents subagents (main agent only).
const agentsPrompt = `## Subagents
agents.spawn({task, cwd?, provider?, model?, effort?}) -> {id} starts a subagent and returns at once. The subagent does NOT see this conversation or your state: put everything it needs in task (goal, context, files, constraints, what to report). Restrictions such as 'do not edit files' are instructions you give it, not enforced. It has the ordinary tools, but no askUser, no tasks, no further subagents; it has its own state and plan. agents.list(), agents.status(id), agents.send(id, text) (steers a running agent; continues a completed or failed one; resumes a blocked one), agents.wait(id, {timeout?}) returns when the agent finishes, is blocked or the timeout passes, agents.result(id) -> {id, status, result, error, usage} (result is null until it has one), agents.cancel(id).
Statuses: starting, running, blocked, completed, failed, cancelled, interrupted. You get an event when an agent completes, fails or blocks; answer a blocked agent with agents.send. You stay responsible for the work: read what the agent did, check it against the agreed scope, and only then update your own plan and criteria. sessions.read(id) shows the agent's full conversation.`

// subagentPrompt tells a child who it is.
const subagentPrompt = `## You are a subagent
A parent agent started you for the task in your first message. You cannot ask the user; you have no background tasks and cannot start agents. agents.report(text) sends a short progress note to the parent. agents.block(reason) tells the parent you cannot continue without something; it takes effect when your current tool calls are done, and you then wait for a reply. Your final answer, when you stop calling tools, is your result for the parent: state what you did, what you found and what is unverified. Follow the instructions in the task about what you may change.`

// subagentSessionPrompt is the session section of a child: no rename.
const subagentSessionPrompt = `## Session
- session.models({provider?, refresh?}) -> {models, errors}: models of the enabled providers (null means unknown).
- session.compact(summary, saveLastNMessages): replace older context with your Markdown summary, keeping at least that many recent messages and the current tool batch; it takes effect after this batch. The originals stay retrievable.
- sessions.read(id, {from?, limit?, includeToolOutput?, includeThinking?}) reads your own conversation in bounded pages.`
