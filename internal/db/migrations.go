package db

// baselineVersion is the schema version that baseline creates directly.
const baselineVersion = 19

// migrations[i] upgrades the schema from version baselineVersion+i to the next.
var migrations = []string{
	// 20: providers and MCP servers live in provider.json and mcp.json; sessions
	// no longer record which session started them; the import markers are not read.
	`DROP TABLE IF EXISTS providers; DROP TABLE IF EXISTS mcp_servers;
	ALTER TABLE sessions DROP COLUMN spawned_by;
	DELETE FROM meta WHERE key IN ('format_epoch', 'migrated_sessions_db');`,
}
