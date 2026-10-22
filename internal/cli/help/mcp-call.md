# mcp call

Usage: meno mcp call SERVER TOOL [ARGS]
       meno mcp call SERVER.TOOL [ARGS]

ARGS is one JSON object or one or more key=value pairs. JSON is used for values that parse as JSON. Use meno mcp tools SERVER to list valid tool names.

Print a JSON envelope with text, structuredContent, isError, resultId and artifact paths. Tool-reported errors keep exit code 0; transport and protocol failures do not.
