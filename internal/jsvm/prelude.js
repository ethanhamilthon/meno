// Runs once in every fresh VM, after the Go bindings (__write, fs, sh) are in
// place. It builds the console global. There is no event loop and no web
// fetch: network access is sh(curl) or websearch(query).
(function (g) {
  "use strict";
  // Captured now: the global is deleted at the end.
  var __write = g.__write;

  function def(name, value) {
    Object.defineProperty(g, name, { value: value, writable: true, configurable: true, enumerable: false });
  }

  // ---- console ----
  function fmt(v) {
    if (typeof v === "string") return v;
    if (v === undefined) return "undefined";
    if (v instanceof Error) return v.stack || v.name + ": " + v.message;
    if (typeof v === "function") return String(v);
    if (typeof v === "object" && v !== null) {
      try {
        var s = JSON.stringify(v, null, 2);
        if (s !== undefined) return s;
      } catch (e) {}
    }
    return String(v);
  }
  function log() {
    var parts = [];
    for (var i = 0; i < arguments.length; i++) parts.push(fmt(arguments[i]));
    __write(parts.join(" ") + "\n");
  }
  def("console", { log: log, info: log, debug: log, warn: log, error: log, dir: log });

  // The raw binding is an implementation detail.
  delete g.__write;
})(globalThis);
