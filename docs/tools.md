# The agent's tool

The model has one tool, `js`. **Every call runs in a fresh runtime**: declarations,
`globalThis` changes, patched builtins and `env` edits do not survive the call.
Durable data lives in `state`, files, tasks and the plan. Host functions are
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
| `sh(cmd)`, `fetch`, `env`, `URL`, `console` | as before |
| `state.set/get/has/delete/listKeys/listValues/listEntries` | durable per-session JSON map, strict JSON (errors name the path), 1 MiB per value, 10 MiB per session; committed before `set` returns |
| `session.rename(name)` | at most 6 words |
| `session.models({provider?, refresh?})` | models of enabled providers with known capabilities; `null` means unknown |
| `session.compact(summary, saveLastNMessages)` | applied after the current tool batch; originals stay retrievable |
| `sessions.list/search/read` | all local sessions, paged; see [sessions](sessions.md) |
| `todo.*`, `todo.criteria.*` | the agent's plan and acceptance criteria, visible in the timeline (`:todo` folds it) |
| `tasks.start/list/status/logs/wait/cancel` | background processes, see below |
| `agents.spawn/list/status/send/wait/result/cancel` | subagents, see below |
| `askUser({questions, timeout})` | one form with all questions, see below |
| `mcp.<server>.<tool>(args)`, `mcpResults`, `mcpServers` | MCP, see below |
| `view(path)` | look at a PNG/JPEG |

## Questions

`askUser({questions: [{id, text, type: "single"|"multi"|"text", options: [{id, label}], recommendation}], timeout})`
returns `{answers: [{id, selected, text}]}`. Every question is required and the
user can always add custom text. `Tab`/`Shift+Tab` move focus, `Space` toggles,
`Enter` activates, `[`/`]` change question, clicks work. `Submit` checks every
question and focuses the first unanswered one; `Cancel` makes the call throw
`AskCancelled` (`AskTimeout` after the timeout). `Esc` never closes the form.

## Tasks

`tasks.start(cmd, {cwd?, env?, name?, timeout?, notify?})` starts `/bin/sh -c` in its
own process group with stdin closed and returns `{id}`. Logs are kept on disk, at
most 10 MiB per task (newest kept, lost ranges are reported). When a task ends
the agent is told through the steering queue unless `notify: false` or it already
saw the result in a successful `tasks.wait`. `:tasks` shows the session's tasks,
live logs, and cancels. Cancel sends SIGTERM to the group and SIGKILL after 5 s.
Tasks live only while meno is open; a task found running after a crash is marked
`interrupted` and never restarted.

## Subagents

`agents.spawn({task, cwd?, provider?, model?, effort?})` starts a child session that
sees only the task text, `AGENTS.md` of its directory and the ordinary tools: no
`askUser`, no tasks, no further agents, no renaming. It has its own state and plan.
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
