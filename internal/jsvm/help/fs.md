# fs
- fs.read(path, {startLine?, endLine?}) returns a string.
- fs.inspect(path, {startLine?, endLine?}) returns {text, sha256, startLine, endLine}. The hash covers the whole file. Lines start at 1 and the range is inclusive.
- fs.search(query, {root?, glob?, regex?, caseSensitive?, includeIgnored?, limit?, cursor?}) returns {matches: [{path, line, column, text}], nextCursor, truncated}. It skips git-ignored files unless includeIgnored is true. Continue with nextCursor.
- fs.patch(path, [{oldText, newText}], {expectedHash}) returns {sha256, replacements}. The edits are exact replacements against what fs.inspect showed. Each oldText must occur exactly once. Edits must not overlap. Nothing is written unless all edits apply. A changed file is refused.
- fs.edit(path, old, new) replaces one unique match.
- fs.write(path, text) writes a file.
- fs.glob(pattern) lists matching paths. fs.exists(path) returns a boolean.
