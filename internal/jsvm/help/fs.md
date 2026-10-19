# fs
- fs.read(path, {startLine?, endLine?, offset?, limit?}) returns a string. `offset` and `limit` alias the range fields; do not pass both forms.
- fs.inspect(path, {startLine?, endLine?, offset?, limit?}) returns {text, sha256, startLine, endLine}. The hash covers the whole file. Lines start at 1 and the range is inclusive. `offset` and `limit` alias the range fields; do not pass both forms.
- fs.search(query, {root?, glob?, regex?, caseSensitive?, includeIgnored?, limit?, cursor?}) returns {matches: [{path, line, column, text}], nextCursor, truncated}. It skips git-ignored files unless includeIgnored is true. Continue with nextCursor.
- fs.patch(path, [{oldText, newText}], {expectedHash}) returns {sha256, replacements}. The edits are exact replacements against what fs.inspect showed. Each oldText must occur exactly once. Edits must not overlap. Nothing is written unless all edits apply. A changed file is refused.
- fs.write(path, text, {overwrite?}) writes a file. It refuses to overwrite an existing file unless overwrite is true. On an existing file, the error gives its sha256 and line count; use fs.inspect and fs.patch for a targeted edit.
- fs.glob(pattern) lists matching paths. fs.exists(path) returns a boolean.
