# mcp
Call a tool as mcp.<server>.<tool>({...}). The call is synchronous. It returns {text, structuredContent, content, artifacts, isError, resultId, truncated}.
- text is only the text blocks of the server. Use JSON.parse if you expect JSON.
- structuredContent is the structured data of the server, or null.
- artifacts are files saved on this computer for images and other binary parts.
- isError is true when the tool itself failed. Transport failures throw.
- A large result is cut and truncated is true. Read the rest with mcpResults.read(resultId, {offset, limit}), which returns {text, offset, end, size, nextOffset}. mcpResults.info(resultId) lists the artifacts.
- mcpTools() returns every tool with its full input schema. mcpTools("server") returns the tools of one server.
