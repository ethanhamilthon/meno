// Defines askUser and its exceptions on top of the host function __ask.
(function (g) {
  "use strict";
  var ask = g.__ask;
  delete g.__ask;
  function def(name, value) {
    Object.defineProperty(g, name, { value: value, writable: true, configurable: true, enumerable: false });
  }
  class AskCancelled extends Error {
    constructor(message) {
      super(message || "the user cancelled the questions");
      this.name = "AskCancelled";
    }
  }
  class AskTimeout extends Error {
    constructor(message) {
      super(message || "the user did not answer in time");
      this.name = "AskTimeout";
    }
  }
  def("AskCancelled", AskCancelled);
  def("AskTimeout", AskTimeout);
  def("askUser", function askUser(spec) {
    var r = ask(spec);
    if (r.status === "cancelled") throw new AskCancelled();
    if (r.status === "timeout") throw new AskTimeout("the user did not answer within " + r.seconds + "s");
    return { answers: r.answers };
  });
})(globalThis);
