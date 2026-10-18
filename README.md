# meno

A terminal coding agent for individual developers. It has one tool, `js`: a
fresh JavaScript runtime ([goja](https://github.com/dop251/goja)) per call with
the file system, a shell, web search, background tasks,
subagents and your MCP servers inside it. The UI is a vim-style TUI built on
[tcell](https://github.com/gdamore/tcell). Pure Go, no CGO. macOS and Linux
(arm64, amd64). MIT licensed.

**There is no sandbox.** The agent runs with your permissions: `fs` reads and
writes any path, `sh` runs any command, and `websearch` or `sh(curl)` reaches
any URL. Run it where you would run a shell you trust the model with.

## Install

macOS and Linux, arm64 and amd64. The installer downloads the release archive for
your machine, checks its SHA-256 against the release's `SHA256SUMS` and puts `meno`
in `~/.local/bin` (`/usr/local/bin` when run as root):

```sh
curl -fsSL https://github.com/ethanhamilthon/meno/releases/latest/download/install.sh | sh
curl -fsSL https://github.com/ethanhamilthon/meno/releases/latest/download/install.sh | sh -s -- v0.2.0   # a version
```

Options (environment): `MENO_INSTALL_DIR`, `MENO_REPO`, `MENO_BASE_URL`. Update with `meno update`
(or run the installer again). meno checks for a new release when it starts and says so on the
welcome screen; set `MENO_NO_UPDATE_CHECK=1` to turn that off. `meno -v` prints the version and
`meno -h` the usage. Remove with `uninstall.sh` (`--purge` also deletes `~/.meno`).
You can also download an archive from the
[releases page](https://github.com/ethanhamilthon/meno/releases) and verify it with
`sha256sum -c SHA256SUMS`.

From source (Go 1.27+):

```sh
git clone https://github.com/ethanhamilthon/meno.git && cd meno
make build && ./meno        # or: make install
```

## Releasing

Push a tag: `git tag v0.2.0 && git push origin v0.2.0`. The `release` workflow runs
`make check`, builds the four archives with `scripts/release.sh`, verifies the checksums
and publishes a GitHub release with the archives, `SHA256SUMS`, `install.sh` and `uninstall.sh` attached
(tags containing `-`, such as `v0.2.0-rc1`, are pre-releases). The `ci` workflow tests
every push and pull request on Linux and macOS and installs a snapshot build with
`install.sh`. Run `scripts/release.sh v0.0.0-local dist` to try it locally.

## First start

1. `:providers` — add a provider: a name, base URL (for example
   `https://api.openai.com/v1`), API key and protocol (`responses` or
   `chat completions`).
2. `:model` — pick a model (the list comes from the provider's `/models`) and an
   effort level. It applies to the current session.
3. Press `i`, type a task, press Enter.

## Documentation

| | |
| --- | --- |
| [The agent's tool](docs/tools.md) | every global the model can use: fs, sh, websearch, session, tasks, agents, askUser, mcp, help |
| [Sessions, history and events](docs/sessions.md) | :new, :fork, :cd, compaction, steering, :stop |
| [Interface](docs/ui.md) | keys, commands, timeline, Markdown, images, JS display |
| [Settings](docs/settings.md), [notifications](docs/notifications.md) | |
| [Upgrading, data and privacy](docs/upgrading.md) | migrations, where data lives, what is logged |
| [Terminal graphics](docs/graphics.md) | Kitty, Sixel and iTerm images |
| [Progress and known limits](docs/progress.md) | what is verified and what is not |

Instructions for the agent live in `$MENO_HOME/AGENTS.md` (global) and
`AGENTS.md` in the session's directory (project); both go into the system
prompt. Skills are markdown files in `.meno/skills/` or `~/.meno/skills/`
(`name.md` or `name/SKILL.md`, optional `name`/`description` header); enabled
skills are listed in the prompt and `/name` in a message asks for one.

## Files

Everything lives in `$MENO_HOME` (default `~/.meno`; `make run` uses `~/.meno-dev`, so a dev build does not mix with an installed one):

| | |
| --- | --- |
| `meno.db` | SQLite (mode 0600): settings, providers (**API keys in plain text**), MCP servers, skill state, snippets, model catalog cache, sessions, history, tasks, agents |
| `attachments/` | image bytes, named by hash |
| `tasks/` | bounded logs of background tasks |
| `mcp-results/` | complete MCP tool results and their binary artifacts, per session |
| `skills/`, `AGENTS.md` | global skills and instructions |
| `provider.json`, `mcp.json`, `theme.json` | optional: add providers, MCP servers and themes, see [settings](docs/settings.md) |

## Development

```sh
make test    # go test ./...
make race    # go test -race ./...
make check   # gofmt + vet + race
make term    # end-to-end: the real binary in a pty against a fake provider
make bench   # deterministic benchmarks (docs/performance.md)
```

See [AGENTS.md](AGENTS.md) for the repository map. `make term` builds the binary,
runs it in a pseudo-terminal, reads the screen through a terminal emulator and
serves the model from a fake provider that speaks both protocols.
