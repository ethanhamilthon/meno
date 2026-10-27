# doctor

Usage: meno doctor [--probe] [--json]

Check the setup and report what is wrong. Read-only: it never logs in, restarts, repairs or changes anything; every finding with a problem carries a suggested fix.

Checks: configuration files, database integrity, providers, MCP servers.

--probe also makes bounded network calls (model lists of the enabled providers). Without it no provider is contacted.

Secrets are never printed. Exit codes: 0 healthy, 4 warnings, 1 errors.
