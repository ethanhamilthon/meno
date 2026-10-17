// Runs once in every fresh VM, after the Go bindings (__write, __fetch,
// __parseURL, env, fs, sh) are in place. It builds the web-style globals the
// model expects: console, URL, URLSearchParams, Headers, Response and fetch.
// Everything here is synchronous; there is no event loop.
(function (g) {
  "use strict";
  // Captured now: the globals are deleted at the end.
  var __write = g.__write, __fetch = g.__fetch, __parseURL = g.__parseURL;

  function def(name, value) {
    Object.defineProperty(g, name, { value: value, writable: true, configurable: true, enumerable: false });
  }
  function hide(obj, name, value) {
    Object.defineProperty(obj, name, { value: value, writable: true, configurable: true, enumerable: false });
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

  // ---- URLSearchParams ----
  function enc(s) {
    return encodeURIComponent(s)
      .replace(/%20/g, "+")
      .replace(/[!'()~]/g, function (c) {
        return "%" + c.charCodeAt(0).toString(16).toUpperCase();
      });
  }
  function dec(s) {
    s = s.replace(/\+/g, " ");
    try {
      return decodeURIComponent(s);
    } catch (e) {
      return s;
    }
  }

  class URLSearchParams {
    constructor(init) {
      hide(this, "_l", []);
      hide(this, "_u", null);
      if (init === undefined || init === null) return;
      if (init instanceof URLSearchParams) {
        this._l = init._l.map(function (p) { return [p[0], p[1]]; });
      } else if (typeof init === "string") {
        this._parse(init);
      } else if (typeof init[Symbol.iterator] === "function") {
        for (var p of init) this._l.push([String(p[0]), String(p[1])]);
      } else {
        for (var k of Object.keys(init)) this._l.push([k, String(init[k])]);
      }
    }
    _parse(s) {
      this._l = [];
      if (s.charAt(0) === "?") s = s.slice(1);
      if (s === "") return;
      for (var part of s.split("&")) {
        if (part === "") continue;
        var i = part.indexOf("=");
        this._l.push(i < 0 ? [dec(part), ""] : [dec(part.slice(0, i)), dec(part.slice(i + 1))]);
      }
    }
    _sync() {
      if (this._u) this._u._setQuery(this.toString());
    }
    get size() { return this._l.length; }
    append(k, v) { this._l.push([String(k), String(v)]); this._sync(); }
    delete(k) {
      k = String(k);
      this._l = this._l.filter(function (p) { return p[0] !== k; });
      this._sync();
    }
    get(k) {
      k = String(k);
      for (var p of this._l) if (p[0] === k) return p[1];
      return null;
    }
    getAll(k) {
      k = String(k);
      return this._l.filter(function (p) { return p[0] === k; }).map(function (p) { return p[1]; });
    }
    has(k) {
      k = String(k);
      return this._l.some(function (p) { return p[0] === k; });
    }
    set(k, v) {
      k = String(k); v = String(v);
      var found = false, out = [];
      for (var p of this._l) {
        if (p[0] !== k) out.push(p);
        else if (!found) { found = true; out.push([k, v]); }
      }
      if (!found) out.push([k, v]);
      this._l = out;
      this._sync();
    }
    sort() {
      this._l = this._l
        .map(function (p, i) { return [p, i]; })
        .sort(function (a, b) { return a[0][0] < b[0][0] ? -1 : a[0][0] > b[0][0] ? 1 : a[1] - b[1]; })
        .map(function (x) { return x[0]; });
      this._sync();
    }
    forEach(fn, thisArg) {
      for (var p of this._l.slice()) fn.call(thisArg, p[1], p[0], this);
    }
    keys() { return this._l.map(function (p) { return p[0]; })[Symbol.iterator](); }
    values() { return this._l.map(function (p) { return p[1]; })[Symbol.iterator](); }
    entries() { return this._l.map(function (p) { return [p[0], p[1]]; })[Symbol.iterator](); }
    [Symbol.iterator]() { return this.entries(); }
    toString() {
      return this._l.map(function (p) { return enc(p[0]) + "=" + enc(p[1]); }).join("&");
    }
  }

  // ---- URL ----
  var special = { "http:": 1, "https:": 1, "ws:": 1, "wss:": 1, "ftp:": 1 };

  class URL {
    constructor(href, base) {
      var r = __parseURL(String(href), base === undefined ? "" : String(base));
      if (!r) throw new TypeError("Invalid URL: " + href);
      hide(this, "_r", r);
      var sp = new URLSearchParams(r.search);
      sp._u = this;
      hide(this, "_sp", sp);
    }
    static canParse(href, base) {
      return !!__parseURL(String(href), base === undefined ? "" : String(base));
    }
    _setQuery(q) { this._r.search = q === "" ? "" : "?" + q; }
    get protocol() { return this._r.protocol; }
    set protocol(v) { v = String(v); this._r.protocol = v.slice(-1) === ":" ? v : v + ":"; }
    get username() { return this._r.username; }
    set username(v) { this._r.username = String(v); }
    get password() { return this._r.password; }
    set password(v) { this._r.password = String(v); }
    get hostname() { return this._r.hostname; }
    set hostname(v) { this._r.hostname = String(v); }
    get port() { return this._r.port; }
    set port(v) { this._r.port = String(v); }
    get host() { return this._r.hostname + (this._r.port ? ":" + this._r.port : ""); }
    set host(v) {
      var m = /^(.*?)(?::(\d*))?$/.exec(String(v));
      this._r.hostname = m[1];
      this._r.port = m[2] || "";
    }
    get pathname() { return this._r.pathname; }
    set pathname(v) { v = String(v); this._r.pathname = v.charAt(0) === "/" ? v : "/" + v; }
    get search() { return this._r.search; }
    set search(v) {
      v = String(v);
      if (v.charAt(0) === "?") v = v.slice(1);
      this._r.search = v === "" ? "" : "?" + v;
      this._sp._parse(v);
    }
    get hash() { return this._r.hash; }
    set hash(v) {
      v = String(v);
      if (v.charAt(0) === "#") v = v.slice(1);
      this._r.hash = v === "" ? "" : "#" + v;
    }
    get searchParams() { return this._sp; }
    get origin() {
      return special[this._r.protocol] ? this._r.protocol + "//" + this.host : "null";
    }
    get href() {
      var r = this._r, auth = "";
      if (r.username) auth = r.username + (r.password ? ":" + r.password : "") + "@";
      var authority = r.authority ? "//" + auth + this.host : "";
      return r.protocol + authority + r.pathname + r.search + r.hash;
    }
    set href(v) {
      var r = __parseURL(String(v), "");
      if (!r) throw new TypeError("Invalid URL: " + v);
      this._r = r;
      this._sp._parse(r.search);
    }
    toString() { return this.href; }
    toJSON() { return this.href; }
  }
  def("URLSearchParams", URLSearchParams);
  def("URL", URL);

  // ---- Headers / Response / fetch ----
  class Headers {
    constructor(init) {
      hide(this, "_m", {});
      if (!init) return;
      if (init instanceof Headers) init.forEach((v, k) => this.append(k, v));
      else if (typeof init[Symbol.iterator] === "function") for (var p of init) this.append(p[0], p[1]);
      else for (var k of Object.keys(init)) this.append(k, init[k]);
    }
    append(k, v) {
      k = String(k).toLowerCase();
      this._m[k] = k in this._m ? this._m[k] + ", " + String(v) : String(v);
    }
    set(k, v) { this._m[String(k).toLowerCase()] = String(v); }
    get(k) {
      k = String(k).toLowerCase();
      return k in this._m ? this._m[k] : null;
    }
    has(k) { return String(k).toLowerCase() in this._m; }
    delete(k) { delete this._m[String(k).toLowerCase()]; }
    forEach(fn, thisArg) {
      for (var k of Object.keys(this._m).sort()) fn.call(thisArg, this._m[k], k, this);
    }
    entries() {
      return Object.keys(this._m).sort().map((k) => [k, this._m[k]])[Symbol.iterator]();
    }
    keys() { return Object.keys(this._m).sort()[Symbol.iterator](); }
    values() { return Object.keys(this._m).sort().map((k) => this._m[k])[Symbol.iterator](); }
    [Symbol.iterator]() { return this.entries(); }
  }

  class Response {
    constructor(body, init) {
      init = init || {};
      hide(this, "_body", body === undefined || body === null ? "" : String(body));
      this.status = init.status === undefined ? 200 : init.status;
      this.statusText = init.statusText || "";
      this.ok = this.status >= 200 && this.status < 300;
      this.url = init.url || "";
      this.redirected = !!init.redirected;
      this.headers = new Headers(init.headers);
    }
    text() { return this._body; }
    json() { return JSON.parse(this._body); }
    clone() {
      return new Response(this._body, { status: this.status, statusText: this.statusText, url: this.url, headers: this.headers });
    }
  }

  // fetch(url, {method, headers, body, redirect}) blocks and returns a Response
  // whose text() and json() return their values directly (await is optional).
  def("fetch", function fetch(input, init) {
    init = init || {};
    var url = input instanceof URL ? input.href : input && typeof input === "object" && input.url ? input.url : String(input);
    var method = String(init.method || (input && input.method) || "GET").toUpperCase();
    var headers = new Headers(init.headers || (input && input.headers));
    var body = init.body;
    if (body instanceof URLSearchParams) {
      if (!headers.has("content-type")) headers.set("content-type", "application/x-www-form-urlencoded;charset=UTF-8");
      body = body.toString();
    } else if (body !== undefined && body !== null && typeof body !== "string") {
      body = String(body);
    }
    var hdrs = {};
    headers.forEach(function (v, k) { hdrs[k] = v; });
    var r = __fetch(url, method, hdrs, body === undefined || body === null ? "" : body, body !== undefined && body !== null, String(init.redirect || "follow"));
    return new Response(r.body, { status: r.status, statusText: r.statusText, url: r.url, redirected: r.redirected, headers: r.headers });
  });
  def("Headers", Headers);
  def("Response", Response);

  // The raw bindings are an implementation detail.
  delete g.__write;
  delete g.__fetch;
  delete g.__parseURL;
})(globalThis);
