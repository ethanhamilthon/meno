# meno 0.2 plan

Decisions come from the planning talk of 2026-10-18. Docs and code stay in English.

## 1. websearch tool, remove fetch / URL / env
- Add `websearch(query, opts)` as a js host function. Order: native web_search of the provider when it has one; else Brave or Tavily (choice and key in settings); else DuckDuckGo as the keyless fallback.
- Remove `fetch`, `URL`, `URLSearchParams` and the `env` global from the VM (`internal/jsvm/web.go`, `vm.go`, `prelude.js`). Keep `env` option of `tasks.start`.
- Rewrite skills chatgpt-image and antigravity-image to use `sh(curl)`.
- Update `help/js.md`, `internal/prompt/prompt.go`, README and tests (`web_test.go`).

### JS API after 0.2 (to confirm)
- `fs`: read, inspect, search, patch, edit, write, glob, exists
- `sh(cmd)`, `view(path)`, `console.log`, `help(topic)`
- `websearch(query, opts)` (new)
- `session`: models (and the todo binding in `bindTodo`)
- `tasks`: start, list, status, logs, wait, cancel
- `agents`: spawn, list, status, send, wait, result, cancel
- `mcp.<server>.<tool>`, `mcpTools`, `mcpResults.read`, `mcpServers`: list, add, update, remove, restart
- `askUser`, `AskCancelled`, `AskTimeout`
- Removed: `fetch`, `URL`, `URLSearchParams`, `env`

## 2. MENO welcome animation
- The wordmark shape stays (4 letters, 5x7 pixels). The glyph in each lit pixel changes over time (scramble). Keep the background wave.
- Files: `internal/app/welcome.go` (`logoRows`, `drawWelcome`, `waveTicker`). Add a test next to `TestWelcomeWaveAnimates`.

## 3. Remove commands, tabbed settings
- Remove `:cache`, `:notify`, `:attach` (`commands.go`, `diagnostics.go`, `command_notify.go`, `attach_async.go`). Images come in by paste, drag and drop or `@`.
- Move theme into settings; remove `:theme` and `modal_theme.go`.
- Settings get tabs: General (theme, images), Display (js code, output, strings, timing, thinking, turn summary), Notify (notify, sound, volume), Agents (default subagent model), Context (auto compact, js timeout).
- Files: `modal_settings.go`, `overlay.go`, `draw_main.go`, `modal_help.go`.

## 4. askUser without final submit
- single: Enter selects and goes to the next question. multi: space marks, Enter goes on. text: Enter goes on.
- Enter on the last question sends all answers at once. Back with arrow or shift+tab keeps the answer.
- Files: `internal/app/ask*.go`, `internal/jsvm/ask.go`, `help/askUser.md`, tests.

## 5. Snippets
- `:save_snippet <name>` saves the selected text. `:paste_snippet <name>` inserts the text in the composer. Selection works in the composer and in the transcript.
- Storage: global, in `meno.db` (new additive migration in `internal/db/migrations.go`). Persistence code in `internal/config`.
- Manage: `:snippets` modal to list, delete and rename.

## 6. Default models for subagents
- New settings: `subagent_provider`, `subagent_model`, `subagent_effort` (Agents tab).
- `agents.spawn` uses the value of the main agent for model and effort; if it is missing, the default from settings; if that is missing too, inherit from the current agent. Fields fall back one by one.
- Files: `internal/jsvm/agents.go`, `internal/app/children_spawn.go`, `config/settings.go`.

## 7. Notify sound volume
- Setting `notify_volume`: 20, 40, 60, 80, 100 (default 100). Scale the samples in `internal/notify/sounds.go`; preview on change.
- Not used when `notify_command` is custom.

## 8. Fixes from the last sessions
Errors over 5 days: pencil.execute 12, unknown help topic "sh" 8, "already declared" 9, timeouts 8, fs.patch old text 2, fs.write outside root 1.
- Find the reason of "Identifier already declared" and fix it.
- `help()`: aliases and hints for `sh`, `view`, `askUser`, `websearch`.
- `fs.write` does not overwrite a file without a flag.
- Host functions: hash, path, base64 with UTF-8.
- Timeouts: clear message with a hint to use `tasks`.
- `fs.patch`: show where the old text stopped matching.

## Order
1. Item 8 (small, lowers risk), 2. items 3 and 7, 3. item 4, 4. items 6 and 5, 5. items 1 and 2.
Bump `internal/version/VERSION` to 0.2.0 at the end. Run `make check` and `make term`.
