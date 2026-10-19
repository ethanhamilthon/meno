# js
The js tool runs JavaScript in a runtime that lives for your current answer. Variables, functions and classes you declare at the top level of a call are there in the next calls (let, const and class behave like var, so a later call may declare the same name again). When the user sends a new message, the runtime starts clean: keep lasting data in files. Code with a top-level return or await runs inside a function, and what it declares stays local to that call. After a timeout the runtime keeps what it had, but an object the stopped code was building may be half done.
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
- In a mode such as #subagents some globals are removed (for example fs, sh, tasks, mcp): a name you do not have fails with ReferenceError. The description of the js tool lists what is left.
