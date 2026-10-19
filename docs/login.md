# Accounts and providers (`:login`)

`:login` is the one place for providers. The list shows what you have added, then
everything you can add:

| Entry | What it does |
| --- | --- |
| **API key · Responses** | a provider that speaks the OpenAI Responses protocol (OpenAI and compatible APIs) |
| **API key · Chat Completions** | a provider that speaks chat completions (OpenRouter, local servers, others) |
| **Log in · Claude**, **ChatGPT / Codex**, **Antigravity (Google)**, **xAI (Grok)**, **Meta**, **Kimi** | sign in with your own account in the browser |

Keys: `j`/`k` select, `Enter` adds or edits, `Space` switches an entry off or on,
`d` twice removes it (logs an account out), `Esc` closes. A form has no save key: an
entry is saved the moment it is valid, and a field you confirm is saved at once. While
it is incomplete the form says `not saved: …`.

With no working provider meno opens `:login` at start and `Esc` does not close it
(`Ctrl+C` quits). It closes by itself when a provider can serve a request: an API key
provider with a key (or a local URL, or a `run` command), or any enabled account.

## How the browser logins work

OAuth logins go through [cliproxyapi](https://github.com/router-for-me/CLIProxyAPI), a
local helper that meno installs for you. The first login downloads the newest release
for your platform from GitHub, checks its SHA-256 against the release's `checksums.txt`
and keeps it in `$MENO_HOME/cliproxy/`. From then on meno looks for a newer release at
every start (at most every six hours) and downloads it in the background. Releases that
keep the major version are used from the next start; a new major version is only
reported, because it may change the configuration format.

- The helper listens on `127.0.0.1` only, on a port meno picks, and answers only to keys
  meno generated (`$MENO_HOME/cliproxy/state.json`, mode 0600). Its tokens are in
  `$MENO_HOME/cliproxy/auth/` (mode 0600). meno starts it when accounts exist, keeps it
  running when all accounts are off, and restarts it after a failure.
- Your enabled accounts reach the model as one provider called `oauth`. meno builds it from
  the accounts and never saves it; `:model` lists their models.
- In `:login`, select **Login helper: restart** to start it again and reload accounts. The
  entry appears when accounts exist or meno cannot reach the helper. The list shows an
  account's disabled reason when the helper provides one.

- **Claude, ChatGPT/Codex and Antigravity**: meno opens the page in your browser and the
  browser comes back to the helper by itself. While the login lasts the helper listens
  for that return on a fixed local port (all interfaces; only the login that is waiting
  can complete). If the browser runs on another machine, press `p` and paste the
  address the browser ended on.
- **xAI, Meta and Kimi**: meno shows a code; confirm it on the page that opens.
- `y` copies the login address, `o` opens it again, `Esc` cancels.
- Using a subscription this way may be against the terms of the service. That is your
  decision; meno does not check it.

## If the helper does not respond

meno tries to restart the helper when an account action fails. If it still does not respond,
open `:login` and select **Login helper: restart**. The row shows the log file path. meno
tries delayed restarts after unexpected exits. It stops those attempts after five failures
in ten minutes. If an account shows `needs a new login`, select it and press `Space` to
try to enable it, or press `d` twice to remove it and sign in again.
