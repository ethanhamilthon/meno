# mcp call

Usage: meno mcp call SERVER TOOL [ARGS]
       meno mcp call SERVER.TOOL [ARGS]

ARGS is one JSON object or key=value pairs. A value that parses as JSON is typed (id=7 is a
number, ok=true a boolean), anything else is a string. Run meno mcp tools SERVER TOOL for the
arguments the tool takes.

Prints one JSON object:
  text               the text blocks of the result, joined
  structuredContent  the server's structured result, when it sent one
  isError            true when the tool reports a failure (an invalid argument is one)
  resultId           the stored full result: read it with meno mcp results ID
  size               bytes of the stored result
  artifacts          images, audio and files of the result: index, kind, mime, size, path
  notes              anything that could not be kept (too large, undecodable)

A tool that fails still exits 0: check isError and read text. Exit 1 is for an unknown server
or tool (the valid names are printed) and for transport or protocol failures. If a long
result is cut by the output cap of your call, read the rest with meno mcp results.
