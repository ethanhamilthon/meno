# The agent's tool

The model has one tool, `js`. **The runtime lives for one answer of the agent**: variables,
functions and classes declared at the top level of a call are there in the next calls, and a
new message from the user (or a new run of a subagent) starts a clean runtime. Top-level `let`,
`const` and `class` behave like `var`, so a later call can declare a name again; code with a
top-level `return` or `await` runs inside a function and keeps its declarations local.
Durable data lives in files and tasks. `help()` lists the topics; `help("name")` returns the full API of one global (the text is embedded in `internal/jsvm/help/`). Host functions are
synchronous; `await` works on values that are already settled and a promise that
stays pending is an error. The last expression, or an explicit top-level
`return`, is the result: strings as they are, objects and arrays as JSON;
functions, Map, Set and cyclic objects are errors. `console.log` output and the
result share one 64 KB budget; there is a timeout (a setting).

| Global | |
| --- | --- |
| `fs.read(path, {startLine?, endLine?})`, `fs.inspect(...)` | 1-based inclusive lines; `inspect` also returns `{sha256}` of the whole file |
| `fs.search(query, {root?, glob?, regex?, caseSensitive?, includeIgnored?, limit?, cursor?})` | `{matches: [{path, line, column, text}], nextCursor, truncated}`; columns are Unicode scalar positions; skips git-ignored files unless asked |
| `fs.patch(path, [{oldText, newText}], {expectedHash})` | all-or-nothing exact edits against what `inspect` showed; each `oldText` must match once; atomic replace keeping permissions |
| `fs.edit`, `fs.write`, `fs.glob`, `fs.exists` | `edit` needs a unique match and says where the near misses are |
| `sh(cmd)`, `console` | shell and output; there is no `fetch`, `URL`, `URLSearchParams` or `env` global |
| `websearch(query, {count?})` | `{query, backend, answer?, results: [{title, url, snippet}]}`; provider-native search, then Brave or Tavily from settings, then DuckDuckGo; `count` is 1..10 (default 5) |
| `session.models({provider?, refresh?})` | models of enabled providers with known capabilities; `null` means unknown |
| `tasks.start/list/status/logs/wait/cancel` | background processes, see below |
| `agents.spawn/list/status/send/wait/result/cancel` | subagents, see below |
| `askUser({questions, timeout})` | one form with all questions, see below |
| `goal.done(summary)`, `goal.blocked(reason)` | the verdict of a `#goal` run, see [modes](modes.md); they throw elsewhere |
| `mcp.<server>.<tool>(args)`, `mcpResults`, `mcpServers` | MCP, see below |
| `view(path)` | look at a PNG/JPEG |

## Questions

`askUser({questions: [{id, text, type: "single"|"multi"|"text", options: [{id, label}], recommendation}], timeout})`
returns `{answers: [{id, selected, text}]}`. Every question is required and the
user can always add custom text. `Tab`/`Shift+Tab` move focus, `Space` toggles,
`[`/`]` change question, clicks work. Enter selects a single choice, marks a
multi choice or accepts text and goes to the next question; Enter on the last
question sends every answer at once. Going back with the arrow keys or
`Shift+Tab` keeps what was already answered. There is no separate submit step.
`Cancel` makes the call throw `AskCancelled` (`AskTimeout` after the timeout).
`Esc` never closes the form.

## Tasks

`tasks.start(cmd, {cwd?, env?, name?, timeout?, notify?})` starts `/bin/sh -c` in its
own process group with stdin closed and returns `{id}`. The process inherits the
environment of meno; `env` adds or overrides single values. Logs are kept on disk, at
most 10 MiB per task (newest kept, lost ranges are reported). When a task ends
the agent is told through the steering queue unless `notify: false` or it already
saw the result in a successful `tasks.wait`. `:tasks` shows the session's tasks,
live logs, and cancels. Cancel sends SIGTERM to the group and SIGKILL after 5 s.
Tasks live only while meno is open; a task found running after a crash is marked
`interrupted` and never restarted.

## Subagents

`agents.spawn({task, cwd?, provider?, model?, effort?})` starts a child session that
sees only the task text, `AGENTS.md` of its directory and the ordinary tools: no
`askUser`, no tasks, no further agents. Provider, model and effort fall back one
by one: the spawn argument, then the defaults in settings, then this agent's
values.
Children report with `agents.report`, ask for help with `agents.block` and finish by
answering. `:agents` lists only working children of the session; open one to watch
and message it. Their spend is shown apart (`delegated`), unknown prices stay
unknown. Children never appear in `:sessions`.

## MCP

Tool calls return `{text, structuredContent, content, artifacts, isError, resultId,
truncated}`. Complete results are stored under `mcp-results/` and can be read back in
bounded slices with `mcpResults.read(resultId, {offset, limit})`; binary parts are
files listed in `artifacts`. Manage servers with `mcpServers.list/add/update/remove/restart`
(secrets are shown as `***`) or in `:mcp`.
