const Module = require("node:module");
const load = Module.createRequire(__filename);
const again = load;
const build = again("esbuild").build;

build({ entryPoints: ["src/chained.ts"], bundle: true });
