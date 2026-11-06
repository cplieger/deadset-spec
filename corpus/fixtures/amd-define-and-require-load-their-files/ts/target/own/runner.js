(function () {
  // A define this file writes itself loads nothing.
  function define(ids, factory) {
    return factory(ids);
  }
  define(["./skipped"], function () {});
})();
