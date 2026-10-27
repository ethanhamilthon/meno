# Providers (`/login`)

`/login` is the one place for providers. The list shows what you have added, then
everything you can add:

| Entry | What it does |
| --- | --- |
| **API key · Responses** | a provider that speaks the OpenAI Responses protocol (OpenAI and compatible APIs) |
| **API key · Chat Completions** | a provider that speaks chat completions (OpenRouter, local servers, others) |

Adding asks one field at a time in the input row: name, base URL, API key, then `run`
(`Enter` skips it). `Esc` cancels and saves nothing. Editing an entry opens a form.

Keys: `j`/`k` select, `Enter` adds or edits, `Space` switches an entry off or on,
`d` twice removes it, `Esc` closes. A form has no save key: an
entry is saved the moment it is valid, and a field you confirm is saved at once. While
it is incomplete the form says `not saved: …`.

With no working provider meno opens `/login` at start and `Esc` does not close it
(`Ctrl+C` quits). It closes by itself when a provider can serve a request: an API key
provider with a key (or a local URL, or a `run` command).

## Custom providers and local servers

Providers are saved in `$MENO_HOME/provider.json`; you can edit that file by hand.
An `api_key` of `env:NAME` reads the key from the environment of meno. A local server
can use an empty key. An optional `run` shell command starts with meno and is
stopped when meno exits. Meno does not install or update external proxies.
See [settings](settings.md) for the file format and model overrides.

For changes affecting older account logins, see [upgrading](upgrading.md).
