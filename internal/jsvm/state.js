// Runs once in every fresh runtime after the Go bindings. Defines the state
// global. Values are validated as strict JSON and encoded here, by hand, so no
// toJSON method or patched builtin can change what is stored.
(function (g) {
  "use strict";
  var h = g.__state;
  delete g.__state;
  var getProto = Object.getPrototypeOf, getDesc = Object.getOwnPropertyDescriptor;
  var ownKeys = Reflect.ownKeys, isArray = Array.isArray, isFinite_ = Number.isFinite;
  var objProto = Object.prototype, arrProto = Array.prototype;
  var strify = JSON.stringify, parse = JSON.parse;
  var TypeErr = TypeError;

  function fail(path, why) {
    throw new TypeErr("state value at " + path + " " + why);
  }
  function enc(v, path, stack) {
    switch (typeof v) {
      case "string": return strify(v);
      case "boolean": return v ? "true" : "false";
      case "number":
        if (!isFinite_(v)) fail(path, "is not a finite number");
        return v === 0 ? "0" : String(v);
      case "undefined": fail(path, "is undefined"); break;
      case "function": fail(path, "is a function"); break;
      case "symbol": fail(path, "is a symbol"); break;
      case "bigint": fail(path, "is a BigInt"); break;
    }
    if (v === null) return "null";
    if (stack.indexOf(v) >= 0) fail(path, "is a cycle");
    var proto = getProto(v), out = [], i, k, d;
    stack.push(v);
    if (isArray(v)) {
      if (proto !== arrProto) fail(path, "is not a plain array");
      for (i = 0; i < v.length; i++) {
        d = getDesc(v, String(i));
        if (!d) fail(path + "[" + i + "]", "is a hole");
        if (!("value" in d)) fail(path + "[" + i + "]", "is an accessor");
        out.push(enc(d.value, path + "[" + i + "]", stack));
      }
      var extra = ownKeys(v).filter(function (key) {
        return key !== "length" && !(typeof key === "string" && /^(0|[1-9][0-9]*)$/.test(key) && +key < v.length);
      });
      if (extra.length) fail(path, "has non-index property " + String(extra[0]));
      stack.pop();
      return "[" + out.join(",") + "]";
    }
    if (proto !== objProto && proto !== null) fail(path, "is not a plain object (" + (v.constructor && v.constructor.name || "custom prototype") + ")");
    var keys = ownKeys(v);
    for (i = 0; i < keys.length; i++) {
      k = keys[i];
      if (typeof k === "symbol") fail(path, "has a symbol key");
      d = getDesc(v, k);
      if (!d.enumerable) fail(path + "." + k, "is not enumerable");
      if (!("value" in d)) fail(path + "." + k, "is an accessor");
      out.push(strify(k) + ":" + enc(d.value, path + "." + k, stack));
    }
    stack.pop();
    return "{" + out.join(",") + "}";
  }
  function checkKey(k) {
    if (typeof k !== "string" || k === "") throw new TypeErr("state key must be a non-empty string");
  }
  var state = {
    set: function (key, value) { checkKey(key); h.set(key, enc(value, "value", [])); },
    get: function (key) {
      checkKey(key);
      var s = h.get(key);
      return s === null ? undefined : parse(s);
    },
    has: function (key) { checkKey(key); return h.get(key) !== null; },
    delete: function (key) { checkKey(key); return h.delete(key); },
    listKeys: function () { return h.entries().map(function (e) { return e[0]; }); },
    listValues: function () { return h.entries().map(function (e) { return parse(e[1]); }); },
    listEntries: function () { return h.entries().map(function (e) { return [e[0], parse(e[1])]; }); },
  };
  Object.defineProperty(g, "state", { value: Object.freeze(state), writable: false, configurable: false, enumerable: false });
})(globalThis);
