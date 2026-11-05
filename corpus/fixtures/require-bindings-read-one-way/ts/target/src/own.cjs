function require() {
  return { Worker: class {} };
}
const { Worker } = require("node:worker_threads");

new Worker("./src/unthreaded.js");
module.exports = {};
