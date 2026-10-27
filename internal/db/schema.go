package db

// baseline is the schema of the current version: what a database of 0.7.5 looks
// like after the migrations in migrations.go. It includes the turns columns that
// 0.7.x added when the program started. Databases older than 0.7.5 are refused
// (see migrateTo); there is no upgrade path from them.
const baseline = `
CREATE TABLE meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE settings (
	key        TEXT PRIMARY KEY,
	value_json TEXT NOT NULL
);
CREATE TABLE skills_state (
	name    TEXT PRIMARY KEY,
	enabled INTEGER NOT NULL
, autoload INTEGER NOT NULL DEFAULT 0);
CREATE TABLE model_catalog (
	model TEXT PRIMARY KEY,
	data  TEXT NOT NULL
);
CREATE TABLE sessions (
	id         TEXT PRIMARY KEY,
	title      TEXT NOT NULL DEFAULT '',
	cwd        TEXT NOT NULL DEFAULT '',
	provider   TEXT NOT NULL DEFAULT '',
	model      TEXT NOT NULL DEFAULT '',
	effort     TEXT NOT NULL DEFAULT '',
	summary    TEXT NOT NULL DEFAULT '',
	metrics    TEXT NOT NULL DEFAULT '{}',
	updated_at TEXT NOT NULL
, legacy INTEGER NOT NULL DEFAULT 0, kind TEXT NOT NULL DEFAULT 'main', parent_id TEXT NOT NULL DEFAULT '', fork_source TEXT NOT NULL DEFAULT '', fork_boundary INTEGER NOT NULL DEFAULT 0, next_hid INTEGER NOT NULL DEFAULT 0, coverage TEXT NOT NULL DEFAULT '', tool TEXT NOT NULL DEFAULT 'js', no_ask INTEGER NOT NULL DEFAULT 0, system_name TEXT NOT NULL DEFAULT '', system_prompt TEXT NOT NULL DEFAULT '');
CREATE INDEX sessions_updated ON sessions(updated_at DESC);
CREATE TABLE turns (
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
	time         TEXT NOT NULL, hid INTEGER NOT NULL DEFAULT 0, event_id TEXT NOT NULL DEFAULT '', title TEXT NOT NULL DEFAULT '',
	system TEXT NOT NULL DEFAULT '', attachments TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL DEFAULT '',
	elapsed_ms INTEGER NOT NULL DEFAULT 0, limit_ms INTEGER NOT NULL DEFAULT 0, timed_out INTEGER NOT NULL DEFAULT 0, run TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (session_id, seq)
) WITHOUT ROWID;
CREATE TABLE requests (
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	seq        INTEGER NOT NULL,
	provider   TEXT NOT NULL DEFAULT '',
	model      TEXT NOT NULL DEFAULT '',
	usage      TEXT NOT NULL,
	cost       REAL,
	time       TEXT NOT NULL, details TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (session_id, seq)
) WITHOUT ROWID;
CREATE TABLE history (
	session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	hid         INTEGER NOT NULL,
	turn        TEXT NOT NULL,
	archived_at TEXT NOT NULL,
	PRIMARY KEY (session_id, hid)
) WITHOUT ROWID;
CREATE TABLE summaries (
	session_id      TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	rev             INTEGER NOT NULL,
	summary         TEXT NOT NULL,
	covers_through  INTEGER NOT NULL,
	created_at      TEXT NOT NULL,
	PRIMARY KEY (session_id, rev)
) WITHOUT ROWID;
CREATE TABLE kv (
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	ns         TEXT NOT NULL,
	key        TEXT NOT NULL,
	value_json TEXT NOT NULL,
	PRIMARY KEY (session_id, ns, key)
) WITHOUT ROWID;
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
CREATE TABLE session_attention (
 session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
 completed INTEGER NOT NULL DEFAULT 0,
 read_through INTEGER NOT NULL DEFAULT 0
 );
CREATE TABLE request_configs (
 session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
 id TEXT NOT NULL, data TEXT NOT NULL,
 PRIMARY KEY(session_id,id)
 ) WITHOUT ROWID;
`
