const { Worker } = require("node:worker_threads");

new Worker("./src/pool/required.js");
