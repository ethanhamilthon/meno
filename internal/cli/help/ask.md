# ask

Usage: meno ask FILE|-

Read a form JSON document from FILE or stdin, submit it to the session, and print the answer object as JSON.

Form:

    {"questions": [
      {"id": "db", "text": "Which database?", "type": "single",
       "options": [{"id": "pg", "label": "Postgres"}, {"id": "lite", "label": "SQLite"}],
       "recommendation": "optional context shown to the user"},
      {"id": "notes", "text": "Anything else?", "type": "text"}
    ], "timeout": 300}

- questions: 1 to 20 objects; id and text are required; ids are unique.
- type: single, multi or text. single and multi need 1 to 20 options of {id, label}; text takes none.
- recommendation: optional string, shown as context, never preselected.
- timeout: optional seconds; without it the question waits until answered.
- The user can always type a custom answer in addition to the options.
- Unknown fields are rejected.
- Sessions started with --no-ask refuse this command (exit 3); the agent asks in the chat.
