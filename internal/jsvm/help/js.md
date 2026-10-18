# js
The js tool runs JavaScript in a fresh runtime for each call. Nothing you declare or change survives to the next call. Keep lasting data in files.
- The runtime is synchronous. Every host function returns its result directly. Do not use async, await, Promise, or then. A promise that stays pending is an error.
- The call returns the value of the last expression, or of an explicit top-level return. Strings are returned as they are. Objects and arrays are returned as JSON. Functions, Map, Set, and cyclic objects are errors.
- console.log output and the returned value share one 64 KB budget for each call. Each call has a timeout.
- sh(cmd) returns {stdout, stderr, code, timedOut}. It runs a shell command in the working directory. For long commands, use tasks.start.
- view(path) shows a PNG or JPEG file. The image appears as a message right after the call finishes. The call itself returns only a confirmation.
- hash(text, algo?) and hash.sha256(text), hash.sha1(text), hash.md5(text), hash.file(path, algo?).
- path: join, dirname, basename, extname, resolve, relative, clean, isAbsolute, parse, sep.
- base64: encode(str), decode(b64), encodeURL(str), decodeURL(b64), plus btoa(str) and atob(b64) (all UTF-8 safe).
- websearch(query, {count?}) returns {query, backend, answer?, results: [{title, url, snippet}]}. It uses the provider's own search, then the backend from settings, then DuckDuckGo. count is 1..10 (default 5).
- console.log(...) writes to the call output.
- There is no fetch, URL, URLSearchParams or env global. For another HTTP API use sh(curl ...).
- Relative paths resolve against the working directory.
