# js
The js tool runs JavaScript in a fresh runtime for each call. Nothing you declare or change survives to the next call. Keep lasting data in files.
- The runtime is synchronous. Every host function returns its result directly. Do not use async, await, Promise, or then. A promise that stays pending is an error.
- The call returns the value of the last expression, or of an explicit top-level return. Strings are returned as they are. Objects and arrays are returned as JSON. Functions, Map, Set, and cyclic objects are errors.
- console.log output and the returned value share one 64 KB budget for each call. Each call has a timeout.
- sh(cmd) returns {stdout, stderr, code, timedOut}. It runs a shell command in the working directory.
- view(path) shows a PNG or JPEG file. The image appears as a message right after the call finishes. The call itself returns only a confirmation.
- fetch(url, init), env (a copy for this call), URL, URLSearchParams, console.log(...).
- Relative paths resolve against the working directory.
