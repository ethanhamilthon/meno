# JSON-RPC over stdio

`meno -p --stdio` serves meno's own protocol on stdin and stdout: JSON-RPC 2.0, one message per
line. It gives a host program nearly everything the UI can do. End of input stops every run and
task of the process and exits. [ACP](acp.md) is a separate adapter for editors.

Requests are `{"jsonrpc":"2.0","id":1,"method":"...","params":{...}}`. Errors carry the engine's
code in `error.data.code` (for example `not_found`, `conflict`, `forbidden`). `--no-ask` creates
sessions without `meno ask`; otherwise a question arrives as an `ask_open` event and is answered
with `session.answer`.

## Methods

| Method | Params | Result |
| --- | --- | --- |
| `hello` | | version and protocol |
| `session.list` | `{cwd?, q?, limit?}` | stored and live sessions |
| `session.create` | `{cwd, model?, effort?, title?, system?, no_ask?}` | the new session |
| `session.get` | `{id}` | snapshot: turns count, running state, open question, live text |
| `session.patch` | `{id, ...}` | rename, model, effort or directory |
| `session.delete` | `{id}` | |
| `session.fork` | `{id, turn?, ...}` | a new session from an earlier message |
| `session.turns` | `{id, from?, limit?}` | a page of the history |
| `session.message` | `{id, text, ...}` | sends or queues a message |
| `session.stop`, `session.compact` | `{id}` | |
| `session.answer` | `{id, ask_id, answers?, cancel?}` | answers an open question |
| `session.resource` | `{id, method?, group, op?, query?, data?}` | any session route of the engine |
| `task.list`, `task.status`, `task.logs`, `task.wait`, `task.cancel` | `{session, ...}` | background tasks of a session; the other params are those of `meno tasks` |
| `subscribe` | `{session?, after_seq?}` | `{subscription}`; events follow as notifications |
| `unsubscribe` | `{subscription}` | |
| `config.get`, `settings.get`, `settings.patch` | | settings |
| `providers.list`, `.create`, `.update`, `.delete`, `.enable` | | API keys are never returned |
| `skills.list`, `skills.enable` | | |
| `mcp.list`, `.add`, `.remove`, `.restart` | | |
| `models.list`, `fs.search` | | |

## Events

After `subscribe` the server sends notifications
`{"method":"event","params":{"subscription":"...","event":{"seq":N,"type":"text_delta",...}}}`.
Without `session` the subscription is the stream of session changes. Call `session.get` first and pass
its `seq` as `after_seq` to see every event after that snapshot; a client that reconnects passes the
last `seq` it saw. A run ends with the event `done`, `error` or `aborted`.
