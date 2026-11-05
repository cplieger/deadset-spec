require("./polyfill");
const helpers = require("./helpers");
require("../lib/loader.js");

helpers.used();

// Nothing calls later, so the file it requires never loads.
function later(): void {
  require("./lazy");
}

export {};
