# meno

A terminal coding agent for individual developers. It has one tool, `bash`;
the `meno` command inside it gives the agent background tasks, images, questions to the user and
your MCP servers. A background daemon owns sessions and runs, so work goes on when you close
the terminal. The UI is a terminal UI built on
[tcell](https://github.com/gdamore/tcell). Pure Go, no CGO. macOS and Linux
(arm64, amd64). MIT licensed.

**There is no sandbox.** The agent runs with your permissions: `bash` runs any
command, so it reads and writes any path and reaches any URL. Run it where you would run a shell you trust the model with.

## Install

macOS and Linux, arm64 and amd64. The installer downloads the release archive for
your machine, checks its SHA-256 against the release's `SHA256SUMS` and puts `meno`
in `~/.local/bin` (`/usr/local/bin` when run as root):

```sh
curl -fsSL https://github.com/ethanhamilthon/meno/releases/latest/download/install.sh | sh
curl -fsSL https://github.com/ethanhamilthon/meno/releases/latest/download/install.sh | sh -s -- v0.2.1   # a version
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

Push a tag: `git tag v0.2.1 && git push origin v0.2.1`. The `release` workflow runs
`make check`, builds the four archives with `scripts/release.sh`, verifies the checksums
and publishes a GitHub release with the archives, `SHA256SUMS`, `install.sh` and `uninstall.sh` attached
(tags containing `-`, such as `v0.2.1-rc1`, are pre-releases). The `ci` workflow tests
every push and pull request on Linux and macOS and installs a snapshot build with
`install.sh`. Run `scripts/release.sh v0.0.0-local dist` to try it locally.

## First start

1. `/login` — meno opens it by itself when there is no provider, and it stays open
   until one works. Add an **API key** provider (name, base URL such as
   `https://api.openai.com/v1`, key; choose *Responses* or *Chat Completions*).
   Local servers and custom proxies work too (see [providers](docs/login.md)).
   Everything you add stays in the list; `space`
   switches an entry off and on, `d` removes it.
2. `/model` — pick a model (the list comes from the provider's `/models`) and an
   effort level. It applies to the current session.
3. Type a task in INPUT and press Enter.

Commands and agent questions open panels above the composer, leaving the
conversation visible. `/settings` is one list; edit `~/.meno/theme.json` for live
palette changes. Unused daemons exit after a 10-second grace period and restart
on the next launch without losing saved sessions.

## Documentation

| | |
| --- | --- |
| [The agent's tool](docs/tools.md) | the `bash` tool and the `meno` command the model uses: files, tasks, mcp, view, ask |
| [Command line](docs/cli.md), [daemon](docs/daemon.md) | `meno sessions`, `send`, `tail`, `doctor`; the background daemon |
| [ACP agent](docs/acp.md) | `meno acp`: use Meno from any ACP client |
| [Sessions, history and events](docs/sessions.md) | /new, /fork, /cd, compaction, steering, /stop |
| [Providers](docs/login.md) | `/login` |
| [Interface](docs/ui.md) | keys, commands, timeline, Markdown, images, tool-call display |
| [Settings](docs/settings.md), [notifications](docs/notifications.md) | |
| [Upgrading, data and privacy](docs/upgrading.md) | migrations, where data lives, what is logged |

Instructions for the agent live in
`AGENTS.md` in the session's directory; it goes into the system
prompt. Skills are markdown files in `.meno/skills/` or `~/.meno/skills/`
(`name.md` or `name/SKILL.md`, optional `name`/`description` header); enabled
skills are listed in the prompt and `/skill:<name>` in a message asks for one.

## Files

Everything lives in `$MENO_HOME` (default `~/.meno`; `make run` uses `~/.meno-dev`, so a dev build does not mix with an installed one):

| | |
| --- | --- |
| `meno.db` | SQLite (mode 0600): settings, providers (**API keys in plain text**), MCP servers, skill state, model catalog cache, sessions, history, tasks |
| `attachments/` | image bytes, named by hash |
| `tasks/` | bounded logs of background tasks |
| `mcp-results/` | complete MCP tool results and their binary artifacts, per session |
| `skills/` | global skills |
| `provider.json`, `mcp.json`, `theme.json` | providers and MCP servers (edited by `/login`, `/mcp` or by hand) and the live palette, see [settings](docs/settings.md) |

## Upgrading to 0.5.0

0.5.0 starts with a clean database and preserves the old database in
`$MENO_HOME/backups/`. History and database-backed accounts/settings are not
automatically transferred. See [upgrading](docs/upgrading.md) before updating.

## Development

```sh
make test    # go test ./...
make race    # go test -race ./...
make check   # gofmt + vet + race
make term    # end-to-end: the real binary in a pty against a fake provider
make bench   # deterministic benchmarks (docs/performance.md)
make clean-home  # delete the dev MENO_HOME (~/.meno-dev)
```

See [AGENTS.md](AGENTS.md) for the repository map. `make term` builds the binary,
runs it in a pseudo-terminal, reads the screen through a terminal emulator and
serves the model from a fake provider that speaks both protocols.
