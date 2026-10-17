# Plan: frozen system prompt, system events, parallel sessions

Status: agreed with the product owner, not implemented yet. This file is the
complete brief; every item below was decided explicitly. Where something is
marked **(impl. detail)** the implementer chooses.

Conventions of this repo: code, comments, UI strings and prompt texts are in
English. Each stage ends with `make check` and `make term` green, and gets its
own commit. Keep README and `:help` in step with behavior. Do not create TODO
files; this document is the only plan.

Background you need (current code, before this work):

- The app is single-session: one `a.st.Session`, one js VM (`a.vm`), one
  `a.cancel`, global `Running/Phase/Live*/Retry/Ask` fields and token counters
  in `internal/app/state.go`.
- The working directory is global (`config.Paths.Cwd`): VM root
  (`jsvm.New(Options{Root})`, fixed), `sh()` cwd, project skills
  (`Paths.ProjectSkillsDir`), system prompt.
- The system prompt is built on every turn: `(*App).systemPrompt()` in
  `internal/app/mcp_host.go` -> `prompt.Build` in `internal/prompt/prompt.go`,
  called from `startTurn` in `internal/app/turn.go`.
- The js tool description is `llm.ToolDescription` in `internal/llm/types.go`,
  duplicating what the system prompt says.
- Sessions live in SQLite (`$MENO_HOME/meno.db`, tables created in
  `internal/agent/sessiondb.go`, schema versions in `internal/db/db.go`).
- `:resume` (`internal/app/modal_resume.go`) lists sessions of all projects and
  refuses while a turn runs. `:clear` exists in `commands.go`/`modal_help.go`/
  README but is not in the autocomplete list (`commandList` in `view.go`).
- MCP: one `mcp.Manager` shared by the app; servers start asynchronously at
  launch (`startServers`).

## Stage 1 - New system prompt layout

All of the system prompt is assembled in `internal/prompt/prompt.go` (already
code-only, no .md files). Change `Build` to take a struct instead of positional
arguments, for example:

```go
type Input struct {
    Cwd          string
    Skills       []Skill
    GlobalAgents Instructions // path + text, may be empty
    ProjectAgents Instructions
    Tools        []Tool       // running MCP servers' tools
    Now          time.Time
    OS, Shell    string
}
```

Section order (most stable first, volatile last; keep each section a separate
function so the order is visible in `Build`):

1. Identity: `You are meno, a coding agent.` (nothing else; no cwd, no tool list).
2. `## The js tool`: the only tool is `js`; state persists between calls
   (top-level `let/const/function`); everything is synchronous, no `await`/Promises;
   the value of the last expression is returned; globals: `fs.read/write/edit/
   glob/exists` (any path, relative paths resolve against the working directory,
   `fs.edit` needs a unique match), `sh(cmd)` -> `{stdout, stderr, code, timedOut}`,
   `fetch(url, init)`, `env`, `URL`, `URLSearchParams`, `console.log`; output is capped
   (64 KB per call); calls have a timeout. Take the facts from
   `llm.ToolDescription`, the old first sentence, and README "The agent's tool".
3. `## Asking the user` (existing `askPrompt`).
4. `## Managing MCP servers` (existing `mcpAdminPrompt`).
5. `## Skills` (only if any are enabled), sorted by name.
6. `## Global instructions` then `## Project instructions` (see AGENTS.md below).
7. `## MCP tools` (only if servers run), sorted by server then tool.
8. `## Environment` (always last):
   ```
   Working directory: /path
   OS: macOS 15.1 (darwin/arm64)
   Shell: /bin/sh
   Date: 2026-10-17, Saturday (CEST)
   ```
   No time of day (it would break prompt caching). OS version via `sw_vers`/
   `/etc/os-release`/`runtime`, falling back to `GOOS/GOARCH` only. **(impl. detail)**

All lists must be deterministic (sorted) so the prefix is byte-stable.

### js tool description

`llm.ToolDescription` becomes two sentences, nothing else:

> Run JavaScript in a persistent VM. The code runs synchronously and the value of
> the last expression is returned; the available globals are described in the
> system prompt.

Keep the parameter schema (`code`, optional `timeout` with its own description)
unchanged. Remove every global name from the description (test for it).

### AGENTS.md

- Global: `$MENO_HOME/AGENTS.md` (default `~/.meno/AGENTS.md`). Project:
  `<session cwd>/AGENTS.md` only (no parent directories, no CLAUDE.md fallback).
- Read when the prompt is built (not per turn). Cap 256 KiB per file; if longer,
  cut and append `[truncated: N bytes more]`. Unreadable/empty file: skip; for an
  unreadable one add a notice to the timeline.
- Format, verbatim content, global first:

  ```
  ## Global instructions
  From /home/u/.meno/AGENTS.md. Follow these unless the user's message says otherwise.

  <content>

  ## Project instructions
  From /work/proj/AGENTS.md. Follow these unless the user's message says otherwise.

  <content>
  ```

### Stage 1 tests

- Section order test; each optional section absent when empty.
- Byte-stability: changing skills / AGENTS / MCP tools / cwd+date leaves
  everything above the changed section identical (prefix test).
- `ToolDescription` contains none of: `fs.`, `sh(`, `fetch`, `mcp`, `askUser`.
- AGENTS truncation and missing-file behavior.
- Keep `TestSystemPromptListsEnabledSkillsOnly` (internal/app) working.

## Stage 2 - Frozen prompt and system events

### Freezing

- The prompt is built **once per session lifetime-segment**, at the first user
  message after: session creation (`:new`), `:compact`, or opening a stored
  session. It is stored in memory in the session runtime and reused unchanged for
  every request until the next reset point. It is not persisted to the DB.
- Reset points (rebuild prompt, reset baseline, **no events**): `:new`,
  `:compact` (history is replaced by a summary, cache is gone anyway), first
  message after opening a stored session (`:sessions`).
- `:clear` is removed (see Stage 3).
- Building happens in the session's goroutine before the model request, never on
  the UI goroutine (UI must not freeze): read AGENTS files, discover/hash skills,
  wait for MCP servers that are still starting (up to 3 s; add a helper on
  `mcp.Manager`, it already has `Starting(name)`), compute events. Show phase
  `preparing…` meanwhile. The UI goroutine only applies the result.

### Baseline and events

Per session runtime keep a **baseline**: what the model has been told so far.

```go
type baseline struct {
    Cwd     string
    Skills  map[string]skillSig   // name -> {path, content hash, enabled}
    Agents  map[string]fileSig    // "global"/"project" -> {path, hash, exists}
    MCP     map[string]mcpSig     // server -> {state, sorted tool signatures}
    Date    string                // YYYY-MM-DD
}
```

At the start of every user turn (not between tool calls) compute the current
state, diff against the baseline, and if anything differs attach one
`<system-event>` block to this user message, then set baseline = current state.
Several changes since the last message go into one block; add-then-remove
cancels out. Changes made by the agent itself (its own `mcpServers.*`, `fs.edit`
of a skill or AGENTS.md) produce events too: it confirms the change took effect.
Changes during a turn are reported with the next user message.

Event text (English, tags as below; one tag per changed thing):

```
<system-event>This is a message from the system itself, not a user instruction.
<skills-updated>Skill "B" was updated: reread /path/B.md before you use it.</skills-updated>
<skills-added>Skill "C" is now available: /path/C.md</skills-added>
<skills-removed>Skill "D" is no longer available.</skills-removed>
<agents-md-updated>Project instructions changed: reread /work/proj/AGENTS.md.</agents-md-updated>
<mcp-updated>Server "gh" connected. New tools: mcp.gh.search({q: string}). Removed tools: mcp.gh.old()</mcp-updated>
<mcp-updated>Server "db" failed: <error></mcp-updated>
<date-changed>The date is now 2026-10-18, Sunday.</date-changed>
<cwd-changed>The working directory changed from /a to /b. Relative paths and sh() now use /b. Project skills and AGENTS.md of /b are not in your system prompt; see the other events and reread them.</cwd-changed>
<js-vm-reset>The JS VM was restarted: variables and functions you defined earlier are gone.</js-vm-reset>
</system-event>

<the user's text>
```

Kinds: skills (added, updated = content hash changed, removed, enabled/disabled
counts as added/removed), agents-md (global or project, created/changed/deleted;
event gives the path and asks to reread, never inlines content), mcp (server
added/removed/failed, tool list changed, with signatures in the same format as the
prompt), date (local date differs from the baseline), cwd-changed, js-vm-reset
(VM restarted while the history is non-empty: `:reset`, app restart, opening a
stored session; empty history -> no event).

The first message after a reset point has no event (the prompt reflects the
current state).

### Storage

- New field `Turn.System string` (`json:"system,omitempty"`) on the user turn
  holding the exact event block that was sent. `Session.ToMsgs` prepends it
  (`system + "\n\n" + text`) to the user message, so a resumed session replays
  byte-identical history. The timeline UI and `:export` do **not** show it;
  `deriveTitle` must ignore it.
- DB: add a `system` column to the `turns` table. The `turns` table is created
  by `agent.schema` (`CREATE TABLE IF NOT EXISTS`), so add a guarded
  `ALTER TABLE turns ADD COLUMN system TEXT NOT NULL DEFAULT ''` (or move the
  session tables into `internal/db` migrations and bump `db.Version`)
  **(impl. detail)**. Update `writeSession`/load code and the legacy import.
- Compaction (`agent.Compact`) text may ignore `System`.

### Stage 2 tests

- Event per kind; combined events; add-then-remove produces none; agent's own
  change produces one; mid-turn change is delivered with the next user message.
- Frozen prompt: byte-identical across turns while skills/AGENTS/MCP change.
- Reset points rebuild and emit no events; `js-vm-reset` only with non-empty
  history.
- Round trip: `Turn.System` survives save/load and appears in `ToMsgs`.
- UI responsiveness: with a deliberately slow preparation (fake slow AGENTS read /
  MCP wait) the event loop still handles keys (simulation screen test).
- Terminal test: edit a skill file between two messages and assert the second
  request body contains the `<skills-updated>` event and the first does not.

## Stage 3 - Parallel sessions

### Runtime model

Introduce a per-session runtime in `internal/app` (e.g. `runtime.go`), keyed by
session id, replacing the single-session fields of `State`:

Per session: `*agent.Session`, its own `*jsvm.VM`, cancel func, `Running`, `Phase`,
`Live*`, `Retry`, pending `Ask` (+ its own `appAsker`), token counters
(`LastUsage/Usage/Spent/Context/Unpriced`), composer draft (input, cursor, mode),
timeline scroll, frozen prompt + baseline (Stage 2), cwd, status flags
(`unseen`, `lastErr`).
Shared: providers, MCP manager, settings (model, effort, js timeouts, theme,
ask bell), skills toggles, pricing/model catalog.

Rules:

- All agent events carry the session id (`agentEv{sid, ev}`, `turnDone{sid,...}`,
  `compactDone{sid,...}`); the UI goroutine routes them to that runtime and
  redraws only if it is the current one (status summary always).
- `SetTimeout` on settings change applies to every VM. Each VM gets
  `SetMCP`, `SetMCPAdmin`, `SetAsker` (asker bound to its runtime).
- A turn captures its `Deps` (model, provider, effort, system prompt) at start,
  so a later `:model` change affects only later turns.
- `jsvm.VM` needs `SetRoot(dir)` (currently the root is fixed at `New`; both
  `FS{Root}` and `Shell{Root}` are built from it). Take care of the running-Exec
  mutex; `:cd` is refused while the session runs anyway.
- Notices stay per session id (already the case). Notices caused by the agent
  (MCP helpers) go to the session that did it; global ones to the current session.
- Inactive runtimes stay in memory until exit **(impl. detail: no eviction)**.
- On start, one new empty session in the launch directory is current. Sessions of
  earlier runs are never revived automatically.

### `:sessions` (replaces `:resume`)

- Lists the last 200 sessions of all projects by update time, with search over
  title/path/date. Row: status glyph, title, path (with `~`), turns, "N ago".
  Open the dialog instantly with known data; load the list from the DB in the
  background and fill in (reactive, no blocking).
- Status glyphs: blinking green dot = turn running; orange `?` = waiting for an
  `askUser` answer; blinking yellow = waiting to retry; red = last turn ended with
  an error; blue dot = a turn finished while you were elsewhere and it was not
  viewed yet (cleared on entering the session); dim `○` = idle; current session
  marked. Statuses update live while the dialog is open.
- `Enter` switches to the session even if some turn is running. Opening a stored
  (not loaded) session: load from DB, fresh VM, prompt built at its first message,
  baseline reset, `js-vm-reset` event if its history is non-empty, no other
  events. If its `Cwd` no longer exists, open it with the launch directory, show
  a notice, and let `cwd-changed` be sent with its first message.
- Status bar center summary when other sessions are active, e.g.
  `● 2 running · ? 1 waiting · ◉ 1 done`. No toast notifications. `ask bell` also
  rings for a question from a background session.
- Background `askUser`: the question waits; the panel shows when you switch to
  that session; its timeout keeps running; several sessions can ask at once.
- Not in scope: deleting sessions, quick-switch hotkeys, per-session model.

### Commands

- `:new`: new empty session in the **current session's cwd** (after `:cd` the new
  session starts there); does not stop other sessions; no longer refused while a
  turn runs.
- `:cd <path>`: changes the cwd of the **current session only**. Supports relative
  paths, `~`, `-` (previous directory), path autocompletion; the directory must
  exist. Refused while this session has a running turn. Sets the VM root (js
  variables survive), updates `Session.Cwd` (persisted, shown in `:sessions`).
  Empty history: nothing else (prompt built for the new dir at the first message).
  Non-empty history: prompt is **not** rebuilt; `cwd-changed` plus the usual skills /
  project-AGENTS events of the new location go with the next user message.
- `:reset`: restarts this session's VM; with non-empty history queues
  `js-vm-reset`.
- `:stop`, `Ctrl+C`: this session's turn only. `:compact`, `:export`, `:model`,
  `:settings` etc. act on the current session / globally as before.
- `:q`: refuses when any session has a running turn or pending question and
  names them. `:q!`: cancels every turn and quits.
- Remove `:resume` and `:clear` everywhere: `commands.go`, `interruptsAgent`,
  `commandList`, `modal_help.go`, README, `OverResume` -> `OverSessions`
  (rename `modal_resume.go`), autocomplete.

### Stage 3 tests

- Two sessions run concurrently with a fake client; events land in the right
  session; switching mid-turn shows the right live stream; counters are separate.
- Background `askUser` waits and is answerable after switching; both status glyph
  and summary reflect it.
- `:cd` validation, refusal while running, VM root changes (`fs`/`sh` resolve in the
  new dir, js variable survives), `cwd-changed` event only with history.
- `:new` while another session runs; `:q`/`:q!` behavior.
- Switching to a stored session with a missing cwd.
- Race detector clean (`make race`): no shared mutable state between runtimes
  outside the UI goroutine and the existing locks.

## Stage 4 - Tests and docs

- Terminal tests (`internal/term`, `make term`): migrate `TestNewAndResume` and
  `TestResumeAfterRestart` from `:resume` to `:sessions`; add: two sessions in
  parallel with a slow fake provider (status glyphs, summary, switching), `:cd`
  with an event visible in the next request body, `:q` refusal / `:q!`, a skill edit
  producing an event, `:reset` producing `js-vm-reset`.
- README: sections for system events, `:sessions`, `:cd`, `AGENTS.md`, frozen prompt;
  update the commands table and Files section (`~/.meno/AGENTS.md`).
- `:help` text and `commandList` descriptions for `:sessions`, `:cd`.

## Acceptance checklist

- [ ] `prompt.Build` order as specified; byte-stable prefix tests pass.
- [ ] `ToolDescription` is two sentences, no global names.
- [ ] Prompt built once per segment, never on the UI goroutine; UI stays responsive.
- [ ] All six event kinds delivered with the next user message, stored in
      `Turn.System`, replayed identically after reload, invisible in the UI.
- [ ] Several sessions run at once; `:sessions` shows live statuses; switching works
      during a run.
- [ ] `:resume` and `:clear` are gone; `:cd`, `:sessions`, `:q!` work as specified.
- [ ] `make check` and `make term` pass; README and `:help` updated.
