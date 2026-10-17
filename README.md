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
   and an effort level.
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
| `mcp.<server>.<tool>({...})`, `mcpTools()` | Tools of running MCP servers, see below. |

Output is capped at 64 KB per call. The timeout is a setting (see below).

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

## Settings (`:settings`)

| Setting | Values |
| --- | --- |
| js code | `all`, `lines <n>`, `off` — how much of the code the timeline shows |
| js output | `all`, `lines <n>`, `off` |
| thinking | `all`, `lines <n>` (the newest rows), `off` |
| js timeout | `<n> sec`, `off`, `model` — with `model` the tool gets a `timeout` argument and the model chooses per call |

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
| `Ctrl+C` | abort the running turn |
| `Ctrl+E/Y/D/U/F/B`, wheel | scroll the timeline |
| `:new` `:resume` `:compact` `:clear` `:export [file]` | sessions |
| `:model` `:settings` `:theme` `:skills` `:mcp` `:providers` | dialogs |
| `:reset` `:stop` `:q` | restart the VM, stop the agent, quit |

Completion lists (`:` commands and `/skill` names) use two columns once they
have six or more entries; Down walks the left column and continues in the right.

## Files

Everything lives in `$MENO_HOME` (default `~/.meno`):

| File | Content |
| --- | --- |
| `meno.db` | SQLite database (mode 0600): settings and theme, providers (**API keys in plain text**), MCP servers, skill on/off state, the model catalog cache and all sessions |
| `skills/` | global skills (markdown files) |

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
setup through the dialogs, streaming, js tool calls, Ctrl+C, `:new`/`:resume`
(also across restarts), resize and narrow terminals, Unicode/Markdown, quitting,
and provider errors with retries. Those tests live in `internal/term` behind
the `termtest` build tag.
