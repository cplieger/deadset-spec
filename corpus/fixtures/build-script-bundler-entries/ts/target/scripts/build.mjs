import { build } from "esbuild";

await build({ entryPoints: ["src/browser.ts"], bundle: true, outfile: "dist/browser.js" });
