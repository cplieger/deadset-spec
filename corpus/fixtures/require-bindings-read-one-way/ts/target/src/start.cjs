const load = require("node:module").createRequire(__filename);
const { Worker } = load("node:worker_threads");

new Worker("./src/thread.js");
require("./own.cjs");
