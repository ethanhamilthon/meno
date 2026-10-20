# Daemon transport

The daemon transport is an early foundation for a later client migration. The
current interactive application still runs in-process.
The client uses a startup lock, starts one daemon, and waits for its socket.

## Commands

- `meno daemon run` runs the local daemon in the foreground.
- `meno daemon start` starts it on demand.
- `meno daemon stop` asks it to stop.
- `meno daemon restart [--force]` stops and starts it. It refuses while
  sessions report active work unless `--force` is set.
- `meno daemon status` reports whether it responds.
- `meno daemon logs [-f]` prints the latest log lines. `-f` prints
  current file content; it does not yet follow new lines.

The daemon uses `$MENO_HOME/run/meno.sock`, a 0700 runtime directory, and a
0600 socket. A short path under the system temporary directory is used when the
normal socket path is too long. Logs are in `$MENO_HOME/log/daemon.log`.

## Transport packages

- `internal/proto` defines JSON request, response, event, and error types.
- `internal/daemon` owns the HTTP server, SSE event hub, and lifecycle.
  Session operations use a `Backend` interface. The command currently uses a
  stub backend, so session endpoints return `not_implemented`.
- `internal/client` provides typed HTTP calls, daemon startup, SSE resume, and a
  view-state mirror.

The protocol number must match exactly. A mismatch instructs the client to run
`meno daemon restart`. Session and global status events use sequence numbers. A client
can fetch a snapshot, then resume the event stream after its sequence number. A stream
that loses too many events asks the client to reload the snapshot.
