package db

// migrations[i] upgrades the schema from version i to i+1. Migrations are
// versioned; the 0.5 format activates after a verified epoch backup.
var migrations = []string{`
CREATE TABLE meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE settings (
	key        TEXT PRIMARY KEY,
	value_json TEXT NOT NULL
);
CREATE TABLE providers (
	pos      INTEGER PRIMARY KEY,
	name     TEXT NOT NULL UNIQUE,
	base_url TEXT NOT NULL DEFAULT '',
	api_key  TEXT NOT NULL DEFAULT '',
	protocol TEXT NOT NULL DEFAULT ''
);
CREATE TABLE mcp_servers (
	pos          INTEGER PRIMARY KEY,
	name         TEXT NOT NULL UNIQUE,
	transport    TEXT NOT NULL DEFAULT '',
	command      TEXT NOT NULL DEFAULT '',
	url          TEXT NOT NULL DEFAULT '',
	enabled      INTEGER NOT NULL DEFAULT 1,
	env_json     TEXT NOT NULL DEFAULT '{}',
	headers_json TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE skills_state (
	name    TEXT PRIMARY KEY,
	enabled INTEGER NOT NULL
);
CREATE TABLE model_catalog (
	model TEXT PRIMARY KEY,
	data  TEXT NOT NULL
);
`,
	`
CREATE TABLE IF NOT EXISTS sessions (
	id         TEXT PRIMARY KEY,
	title      TEXT NOT NULL DEFAULT '',
	cwd        TEXT NOT NULL DEFAULT '',
	provider   TEXT NOT NULL DEFAULT '',
	model      TEXT NOT NULL DEFAULT '',
	effort     TEXT NOT NULL DEFAULT '',
	summary    TEXT NOT NULL DEFAULT '',
	metrics    TEXT NOT NULL DEFAULT '{}',
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_updated ON sessions(updated_at DESC);
CREATE TABLE IF NOT EXISTS turns (
	session_id   TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	seq          INTEGER NOT NULL,
	role         TEXT NOT NULL,
	text         TEXT NOT NULL DEFAULT '',
	thinking     TEXT NOT NULL DEFAULT '',
	call_id      TEXT NOT NULL DEFAULT '',
	code         TEXT NOT NULL DEFAULT '',
	output       TEXT NOT NULL DEFAULT '',
	err          TEXT NOT NULL DEFAULT '',
	replay       TEXT,
	replay_group TEXT NOT NULL DEFAULT '',
	time         TEXT NOT NULL,
	PRIMARY KEY (session_id, seq)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS requests (
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	seq        INTEGER NOT NULL,
	provider   TEXT NOT NULL DEFAULT '',
	model      TEXT NOT NULL DEFAULT '',
	usage      TEXT NOT NULL,
	cost       REAL,
	time       TEXT NOT NULL,
	PRIMARY KEY (session_id, seq)
) WITHOUT ROWID;
`,
	`
-- Pre-existing sessions are flagged legacy: they have no state/TODO journal.
ALTER TABLE sessions ADD COLUMN legacy INTEGER NOT NULL DEFAULT 0;
UPDATE sessions SET legacy = 1;
ALTER TABLE sessions ADD COLUMN kind TEXT NOT NULL DEFAULT 'main';
ALTER TABLE sessions ADD COLUMN parent_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN fork_source TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN fork_boundary INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN next_hid INTEGER NOT NULL DEFAULT 0;
-- Sessions that were compacted by an older version already lost history.
ALTER TABLE sessions ADD COLUMN coverage TEXT NOT NULL DEFAULT '';
UPDATE sessions SET coverage = 'partial' WHERE summary <> '';
ALTER TABLE turns ADD COLUMN hid INTEGER NOT NULL DEFAULT 0;
-- Original turns removed from the active context by compaction. Never deleted
-- except together with the owning session.
CREATE TABLE history (
	session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	hid         INTEGER NOT NULL,
	turn        TEXT NOT NULL,
	archived_at TEXT NOT NULL,
	PRIMARY KEY (session_id, hid)
) WITHOUT ROWID;
-- Every summary the session ever had; covers_through is the last hid it covers.
CREATE TABLE summaries (
	session_id      TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	rev             INTEGER NOT NULL,
	summary         TEXT NOT NULL,
	covers_through  INTEGER NOT NULL,
	created_at      TEXT NOT NULL,
	PRIMARY KEY (session_id, rev)
) WITHOUT ROWID;
-- Summaries written by older versions become revision 0, covering everything
-- before the turns that were kept.
INSERT INTO summaries(session_id,rev,summary,covers_through,created_at)
	SELECT id, 0, summary, 0, updated_at FROM sessions WHERE summary <> '';
-- Current values of the per-session versioned stores (ns: state, todo).
CREATE TABLE kv (
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	ns         TEXT NOT NULL,
	key        TEXT NOT NULL,
	value_json TEXT NOT NULL,
	PRIMARY KEY (session_id, ns, key)
) WITHOUT ROWID;
-- Append-only journal: each mutation with the history boundary it happened at.
CREATE TABLE journal (
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	ver        INTEGER NOT NULL,
	ns         TEXT NOT NULL,
	key        TEXT NOT NULL,
	value_json TEXT,
	deleted    INTEGER NOT NULL DEFAULT 0,
	boundary   INTEGER NOT NULL DEFAULT 0,
	time       TEXT NOT NULL,
	PRIMARY KEY (session_id, ver)
) WITHOUT ROWID;
`,
	`
-- The steering queue: user input forwarded to children, child reports and
-- completions, task completions. State: pending, delivered, acked, cancelled.
CREATE TABLE events (
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	id         TEXT NOT NULL,
	seq        INTEGER NOT NULL,
	source     TEXT NOT NULL,
	from_id    TEXT NOT NULL DEFAULT '',
	kind       TEXT NOT NULL DEFAULT '',
	owner      TEXT NOT NULL DEFAULT '',
	text       TEXT NOT NULL DEFAULT '',
	refs       TEXT NOT NULL DEFAULT '',
	time       TEXT NOT NULL,
	state      TEXT NOT NULL DEFAULT 'pending',
	PRIMARY KEY (session_id, id)
) WITHOUT ROWID;
CREATE INDEX events_pending ON events(session_id, state, seq);
ALTER TABLE turns ADD COLUMN event_id TEXT NOT NULL DEFAULT '';
`,
	`
-- Background processes started by the main agent. Logs live on disk next to
-- the database; a task found running at startup is marked interrupted, never
-- restarted.
CREATE TABLE tasks (
	id         TEXT PRIMARY KEY,
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	owner      TEXT NOT NULL DEFAULT '',
	name       TEXT NOT NULL DEFAULT '',
	command    TEXT NOT NULL,
	cwd        TEXT NOT NULL,
	status     TEXT NOT NULL,
	exit_code  INTEGER,
	reason     TEXT NOT NULL DEFAULT '',
	notify     INTEGER NOT NULL DEFAULT 1,
	timeout_ms INTEGER NOT NULL DEFAULT 0,
	started_at TEXT NOT NULL,
	ended_at   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX tasks_session ON tasks(session_id, started_at);
`,
	`
-- Subagents. The child's conversation is an ordinary session (kind child) that
-- the parent can read by its ID; this table holds the lifecycle and the result.
CREATE TABLE agents (
	id           TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
	parent_id    TEXT NOT NULL,
	owner        TEXT NOT NULL DEFAULT '',
	task         TEXT NOT NULL,
	status       TEXT NOT NULL,
	result       TEXT,
	error        TEXT NOT NULL DEFAULT '',
	block_reason TEXT NOT NULL DEFAULT '',
	gen          INTEGER NOT NULL DEFAULT 1,
	usage        TEXT NOT NULL DEFAULT '{}',
	cost         REAL NOT NULL DEFAULT 0,
	unpriced     INTEGER NOT NULL DEFAULT 0,
	created_at   TEXT NOT NULL,
	ended_at     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX agents_parent ON agents(parent_id, created_at);
`,
	`
-- The state global is gone: its values and their journal are dropped. The
-- plan (ns todo) keeps using the same tables.
DELETE FROM kv WHERE ns = 'state';
DELETE FROM journal WHERE ns = 'state';
`,
	`
-- Providers: a command meno runs for a local proxy, and the user's own settings
-- for single models (JSON object by model id).
ALTER TABLE providers ADD COLUMN run TEXT NOT NULL DEFAULT '';
ALTER TABLE providers ADD COLUMN models_json TEXT NOT NULL DEFAULT '{}';
`,
	`
-- The short title the model gives each js call.
ALTER TABLE turns ADD COLUMN title TEXT NOT NULL DEFAULT '';
`,
	`
-- Named snippets the user saves from the terminal and pastes into the
-- composer. They are global: not tied to a session.
CREATE TABLE snippets (
	name       TEXT PRIMARY KEY,
	text       TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL DEFAULT ''
);
`,
	`
-- A provider can be switched off without losing its settings.
ALTER TABLE providers ADD COLUMN disabled INTEGER NOT NULL DEFAULT 0;
`,
	`
-- Bash tool selection. Sessions that already exist keep their js history.
ALTER TABLE sessions ADD COLUMN tool TEXT NOT NULL DEFAULT 'js';
`,
	`DROP TABLE IF EXISTS snippets;

 INSERT OR REPLACE INTO meta(key,value) VALUES('format_epoch','0.5');
 INSERT OR REPLACE INTO meta(key,value) VALUES('migrated_sessions_db','1');
 CREATE TABLE session_attention (
 session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
 completed INTEGER NOT NULL DEFAULT 0,
 read_through INTEGER NOT NULL DEFAULT 0
 );`,
	`ALTER TABLE requests ADD COLUMN details TEXT NOT NULL DEFAULT '';
 CREATE TABLE request_configs (
 session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
 id TEXT NOT NULL, data TEXT NOT NULL,
 PRIMARY KEY(session_id,id)
 ) WITHOUT ROWID;`,
	`-- A stdio MCP server may name its arguments and its working directory
-- separately, the usual shape of a server configuration.
ALTER TABLE mcp_servers ADD COLUMN args_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE mcp_servers ADD COLUMN cwd TEXT NOT NULL DEFAULT '';
`,
}
