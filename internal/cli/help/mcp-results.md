# mcp results

Usage: meno mcp results ID [--offset N --limit N]

Read a stored result by the resultId that meno mcp call printed. Only the session that made
the call can read it.

The page is 10000 bytes unless --limit asks for another size (at most 65536). The text is a
slice of the stored result JSON, cut on character boundaries. Plain output is the slice alone.
--json prints {"text", "offset", "end", "size", "nextOffset"}: pass nextOffset as --offset to
read on; it is absent on the last page.
