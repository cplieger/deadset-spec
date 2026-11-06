define(["./a", "require", "left-pad"], function (a) {
  a.go();
});

require(["./lib/b", "./own/runner"], function () {});

// Nothing calls later, so the file its AMD require names never loads.
function later() {
  require(["./lazy"], function () {});
}
