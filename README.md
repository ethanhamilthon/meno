# meno

A terminal coding agent. It has one tool: a persistent JavaScript VM
([goja](https://github.com/dop251/goja)) with the file system, a shell, HTTP and
your MCP servers inside it. The UI is a vim-style TUI built on
[tcell](https://github.com/gdamore/tcell). Pure Go, no CGO.

**There is no sandbox.** The agent runs with your permissions: `fs` reads and
writes any path, `sh` runs any command, `fetch` reaches any URL. Run it
where you would run a shell you trust the model with.

## Install and run

Needs Go 1.27 or newer and a Unix-like system (`sh`, process groups).

```sh
make build        # ./meno
./meno            # run in the project directory you want the agent to work on
```

`make install` puts the binary in `$(go env GOBIN)` (or `GOPATH/bin`).

First start:

1. `:providers` — add a provider: a name, base URL (for example
   `https://api.openai.com/v1`), API key and protocol (`responses` or
   `chat completions`).
2. `:model` — pick a model (the list is fetched from the provider's `/models`)
   and an effort level. It applies to the current session; every session has its own.
3. Press `i`, type a task, press Enter.

## The agent's tool

The model gets a single tool, `js`. State persists between calls: top-level
`let`, `const` and `function` stay defined (`:reset` starts a fresh VM).
Everything is synchronous, so there is no `await`.

| Global | |
| --- | --- |
| `fs.read(path)`, `fs.write(path, text)`, `fs.edit(path, old, new)`, `fs.glob(pattern)`, `fs.exists(path)` | Any path; relative paths resolve against the working directory, `~` is the home directory. `fs.edit` fails unless `old` matches exactly once. |
| `sh(cmd)` | Runs `/bin/sh -c`, returns `{stdout, stderr, code, timedOut}`. |
| `fetch(url, init)` | Blocking. Returns a `Response` whose `text()` and `json()` return values directly. |
| `env` | The environment as an object; changes are seen by `sh`. |
| `URL`, `URLSearchParams` | As in browsers. |
| `console.log(...)` | Output (also `info`, `warn`, `error`). The value of the last expression is returned too. |
| `askUser(question, {choices, customAnswer, timeout})` | Asks you something and returns the answer as a string, see below. |
| `mcp.<server>.<tool>({...})`, `mcpTools()` | Tools of running MCP servers, see below. |

Output is capped at 64 KB per call. The timeout is a setting (see below).

## AGENTS.md

Instructions for the agent live in plain markdown files: `$MENO_HOME/AGENTS.md`
(global) and `AGENTS.md` in the session's working directory (project; no parent
directories are searched). Both go into the system prompt, global first, with
their paths, and are read when the prompt is built. A file is cut at 256 KiB; a
missing or empty one is ignored.

## The frozen prompt and system events

The system prompt is built once per segment of a session: when the first message is
sent after `:new`, after `:compact`, or after a stored session is opened. It is then
reused byte for byte; its Environment section names the working directory, OS, date and the session's model (so the provider's prompt cache keeps working) and is not
stored. While it is built (reading `AGENTS.md`, finding skills, waiting up to 3 s for
MCP servers that are still starting) the status shows `preparing...`.

Whatever changes afterwards reaches the model as one `<system-event>` block in front
of your next message: skills added, updated or removed, `AGENTS.md` created, changed
or deleted, MCP servers connected, failed or gone and their tools, a new date, a changed model or effort (`:model`), a changed
working directory (`:cd`), and a restarted JS VM (`:reset`, opening a stored session). Changes the agent makes itself
are reported too. The block is saved with the message, so a resumed session replays
the same history, but it is not shown in the timeline or `:export`.

## Questions from the agent

`askUser("Which database?", { choices: ["sqlite", "postgres"], timeout: 120 })`
pauses the agent's code and opens a panel above the composer (it shows
`waiting for your answer` in the status bar). `1`-`9` or `↑/↓` + `Enter` pick a choice;
the last row, "Your own answer…", moves the answer into the composer (hidden when
`customAnswer: false`). Without `choices` you simply type the answer and press
`Enter`. Your half-typed message is put aside meanwhile and comes back afterwards.
`Ctrl+X` (or `Esc` in a choice list) declines: the code gets the error
`user declined to answer`; an expired `timeout` (seconds, none by default) gives
`user did not answer within Ns`. `Ctrl+C` aborts the whole turn. The `js timeout`
does not run while a question is open. Questions and answers are not stored in the
session. The setting `ask bell` rings the terminal bell when a question opens.

## Provider errors

A failed model request is sent again, up to 5 attempts in total, waiting about
1, 2, 4 and 8 s (a `Retry-After` from the provider wins, at most 60 s). Network
errors, dropped streams, 408, 425, 429 and 5xx are retried; other 4xx answers
(bad key, bad request, unknown model) are not, since sending the same request
again cannot help. The status bar shows `retry 2/5 in 4s — <reason>`, the
timeline gets a line per attempt, and `Ctrl+C` cancels the wait. A request that
broke mid-stream starts over and its partial output is discarded; tools only run
after a complete response. After the last failure the turn ends with the error
and your message stays in the session. (`:compact` does not retry.)

## Markdown in answers

The model's text is drawn for reading, not as raw markdown:

- headings (the first level is underlined), **bold**, *italic*, ~~strikethrough~~, `inline code` on a
  panel, links as blue underlined text with the site after it (`docs ↗ example.com`), images as `▣ alt`;
- bulleted, numbered and nested lists with hanging indents, task lists as `[x]` / `[ ]`;
- quotes with a bar, and GitHub alerts (`> [!NOTE]`, `TIP`, `IMPORTANT`, `WARNING`, `CAUTION`)
  with a coloured label;
- fenced code on a panel: language and file name on top (````go internal/x.go`), line numbers from
  6 lines, colours for go, js/ts, python, rust, c-like, sql, json, yaml and shell; `diff` blocks tint added
  and removed lines;
- tables with aligned columns (`:--`, `--:`, `:-:`) that shrink to the terminal, wrap long cells and
  fall back to one line per row when there is no room;
- text wraps at words; a single newline is kept as a line break; `<br>` breaks a line, HTML comments are
  hidden and control characters are dropped, so model output cannot drive the terminal.

## Timing

Every js call shows how long it took under its code: `✓ 3.2s`. A call that used half of its
timeout (or ran 10 s or more) turns yellow and shows a bar with `57% of 30s limit`; a failed call is `✕ 1.2s`
on a red-tinted background; a call stopped by the timeout reads `◷ timed out after 30s`. While it runs the
line counts up with a spinner. In the status bar a running agent shows `turn 3 · 0:42 · 2 js calls · ↓ 12.4k ↑ 1.1k`
and the composer placeholder carries the clock.

When the agent finishes (or fails, or you stop it) one line sums the run up:
`✓ 1m 24s · 7 turns · 6 js calls · 23s in js ──── ↓48.2k ↑3.1k ↺82% $0.084` — time, requests to the model (turns),
js calls and their time, and on the right tokens in and out, the cache hit rate and the cost (when the model has a
price). `✕ failed after 12s` adds the reason above it; `■ stopped after 8s` marks Ctrl+C. Narrow terminals drop the
right side first. Durations are stored with the session; sessions from before show none.

## Settings (`:settings`)

| Setting | Values |
| --- | --- |
| js code | `all`, `lines <n>`, `off` — how much of the code the timeline shows |
| js output | `all`, `lines <n>`, `off` |
| thinking | `all`, `lines <n>` (the newest rows), `off` |
| js timeout | `<n> sec`, `off`, `model` — with `model` the tool gets a `timeout` argument and the model chooses per call |
| js timing | `all`, `slow only`, `off` — which finished js calls show their duration (slow: half of the limit or 10 s) |
| turn summary | `on`, `off` — the line that closes a run |
| notify | `off`, `question`, `done` — signal when the agent asks a question (`question`), and also when a turn ends, with success or error (`done`); not after your own Ctrl+C. Works for every session, also in the background |

The signal is made by `notify_command`, run with `sh -c` in the background (5 s limit) with
`MENO_NOTIFY_EVENT` (`question`, `done` or `error`), `MENO_NOTIFY_SESSION` and
`MENO_NOTIFY_TITLE` in its environment. By default it plays the Glass sound on macOS and
rings the terminal bell elsewhere; `bell` forces the bell. Set it with
`:notify command <cmd>` (no argument restores the default), the mode with `:notify off|question|done`,
and try it with `:notify test`. An old `notify_question: on` becomes `question`.

`←`/`→` change the choice, `+`/`-` the number. `:theme` opens 20 built-in
color themes with a live preview.

## Skills (`:skills`)

A skill is a markdown file the agent reads when a task matches it. meno looks in

- `.meno/skills/` in the project, and
- `~/.meno/skills/` (global); a project skill replaces a global one with the same name.

Both layouts work: `name.md` and `name/SKILL.md`. An optional header sets the
name and the one-line description the model sees:

```markdown
---
name: api-security
description: check auth, rate limits and input validation on routes
---
# API security
...
```

Without a header the file name is the name and the first line of text is the
description. Enabled skills are listed in the system prompt with their paths;
writing `/name` in a message asks for that skill explicitly. Toggle skills with
Space in `:skills`.

## MCP (`:mcp`)

Add servers in the dialog (`n` new, Enter edit, Space enable/disable, `r`
restart). Transports: `stdio` (command line, shell-style quoting, optional
`env`), `sse` and `streamable-http` (URL, optional `headers` such as
`Authorization=Bearer ...`). Enabled servers start with meno. Their tools are
listed in the system prompt and called from JavaScript:

```js
JSON.parse(mcp.github.search_repositories({ query: "meno" }))
mcpTools() // every tool with its full input schema
```

The agent can also manage the servers itself with `mcpServers`:

```js
mcpServers.list()                       // name, transport, status, tools; env/headers values shown as ***
mcpServers.add({ name: "fs", command: "npx -y @modelcontextprotocol/server-filesystem ." })
mcpServers.update("fs", { enabled: false })
mcpServers.restart("fs"); mcpServers.remove("fs")
mcpTools("fs")                          // tools of one server
```

`add`, `update` and `restart` wait for the server and return `{ok, error, tools}`.
Changes are saved to the database, appear in the timeline as `mcp: agent added …`,
and open `:mcp` dialogs follow them live.

Server names that are not valid identifiers also work with underscores
(`mcp.my_db.run_query`) or brackets (`mcp["my-db"]["run-query"]`).

## Keys

Insert mode types text; `Esc` leaves it for Normal mode, which edits the input
like vim (`d c y` with motions and text objects, `p u ctrl+r`, counts, `f t ; ,`,
`v V ctrl+v`). `k` on the first input row continues into the timeline, which is
read-only but supports every motion and yank (`y` copies through OSC 52).
`:` opens commands; `:help` lists everything.

| | |
| --- | --- |
| `Enter` / `Shift+Enter` | send / new line |
| `Enter` while the agent works | steering: the message waits in a panel above the input and joins the conversation before the next model request (after the current js call); `↑` on an empty input takes it back for editing |
| `Ctrl+C` | abort the running turn of this session; messages that were not delivered return to the input |
| `Ctrl+V`, `:attach [path]`, `p` (Normal mode, empty register) | attach an image from the clipboard (macOS; Linux needs `wl-paste` or `xclip`) or from a file; pasting or dropping a path to a PNG/JPEG works too |
| `Ctrl+E/Y/D/U/F/B`, wheel | scroll the timeline |
| `:new` `:sessions` `:cd <path>` `:compact` `:export [file]` | sessions, see below |
| `:model` `:settings` `:theme` `:skills` `:mcp` `:providers` | dialogs |
| `:reset` `:stop` `:q` `:q!` | restart this session's VM, stop its turn, quit (`:q!` also stops running turns) |

## Images

An attached image shows up in the input as the token `[Image #N]`, which moves, wraps
and is deleted as one unit; the text around it tells the model what to look at. At most 5
images per message, 5 MB each, PNG or JPEG; larger images are scaled down to 1568 px on
the longer side. The bytes are kept in `attachments/` next to `meno.db` (named by
hash). Only the 5 newest images of a conversation are sent with each request; older ones
are replaced by `[image omitted]`. `:compact` gives the summarizer the image paths, and
`:export` links the files. If the catalog says the model has no image input, attaching is
refused; when it does not say, the request is sent anyway.

The agent can look at an image itself: `view(path)` in a js call (same paths as `fs`, same
limits, at most 5 per step) queues the file, and it reaches the model as a separate message
right after the call's result. In the timeline it shows as a dim `◌ Image from view("…")` line.
It counts toward the 5 newest images like any other.

Completion lists (`:` commands and `/skill` names) use two columns once they
have six or more entries; Down walks the left column and continues in the right.

## Sessions

Several sessions can run at the same time. Each has its own conversation, js VM
(variables are not shared), working directory, model and effort, token counters,
pending question and half-typed message; providers, MCP servers and settings are
shared. `:model` changes the model of the current session only (it is saved with the
session) and becomes the default for sessions started later. A turn keeps the model
and effort it started with, so a change only affects later turns; the model is told
with a `model-changed` event.

- `:sessions` lists the last 200 sessions of all projects (search by title, path or
  date) and opens at once; the list fills in from the database. Each row has a
  status: blinking green dot = a turn is running, orange `?` = waiting for your
  answer to `askUser`, blinking yellow = waiting to retry, red = the last turn
  failed, blue = a turn finished while you were elsewhere and you have not seen it
  yet, dim `○` = idle. `Enter` switches, also while turns run. A stored session
  gets a new js VM (the model is told with a `js-vm-reset` event) and its prompt is
  built at its first message. If its directory is gone it opens in the launch
  directory and the model is told about the change.
- While other sessions are active the status bar says so, for example
  `● 2 running · ? 1 waiting · ◉ 1 done`. The ask bell also rings for questions of
  background sessions; such a question waits until you switch to its session.
- `:new` starts an empty session in the current session's directory and leaves
  the others running. Empty sessions you never used are not stored.
- `:cd <path>` changes the working directory of the current session only
  (`~`, relative paths and `-` for the previous directory work; Tab completes
  directories). It is refused while the session runs. `fs` and `sh` use the new
  directory at once, variables survive. With history, the prompt is not rebuilt: the
  next message carries a `cwd-changed` event plus the skills and `AGENTS.md` events
  of the new location.
- `:stop` and `Ctrl+C` stop the current session's turn only. `:q` refuses while any
  session runs or waits for an answer and names it; `:q!` cancels everything and quits.

## Files

Everything lives in `$MENO_HOME` (default `~/.meno`):

| File | Content |
| --- | --- |
| `meno.db` | SQLite database (mode 0600): settings and theme, providers (**API keys in plain text**), MCP servers, skill on/off state, the model catalog cache and all sessions |
| `skills/` | global skills (markdown files) |
| `AGENTS.md` | global instructions for the agent (optional) |

On the first start after an update, the old `settings.json`, `providers.json`,
`mcp.json`, `skills.json`, `model-catalog.json` and `sessions/` are imported
into `meno.db` once. The old files are kept but no longer read.

## Development

```sh
make test    # go test ./...
make race    # go test -race ./...
make vet     # go vet ./...
make fmt     # gofmt -l, fails when something is unformatted
make check   # fmt + vet + race
make term    # end-to-end: the real binary in a pty (needs no network)
```

Layout: `cmd/meno` → `internal/app` (TUI state, dialogs, vim editing) over
`agent` (turn loop, sessions), `llm` (chat completions and responses
streaming), `jsvm` (the VM and its globals), `mcp` (clients), `config`, `db` and
`store` (SQLite, settings, JSON helpers), `ui` (drawing, themes) and `prompt` (the system prompt
texts). Tests drive the real UI on tcell's simulation screen. `make term` goes further:
it builds the binary, runs it in a pseudo-terminal (`creack/pty`), reads the
screen through a terminal emulator (`vt10x`) and serves the model from a fake
provider that speaks both protocols and can fail on demand. Scenarios: first-run
setup through the dialogs, streaming, js tool calls, Ctrl+C, `:new`/`:sessions`
(also across restarts), parallel sessions, `:cd`, system events, `:q`/`:q!`, resize and narrow terminals, Unicode/Markdown, quitting,
and provider errors with retries. Those tests live in `internal/term` behind
the `termtest` build tag.
