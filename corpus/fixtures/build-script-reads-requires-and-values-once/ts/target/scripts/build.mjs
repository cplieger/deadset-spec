import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const esbuild = require("esbuild");

let options = { entryPoints: ["src/created.ts"], bundle: true, outfile: "dist/created.js" };
await esbuild.build(options);

function local(esbuild) {
  esbuild.build({ entryPoints: ["src/shadowed.ts"] });
}
local({ build() {} });
