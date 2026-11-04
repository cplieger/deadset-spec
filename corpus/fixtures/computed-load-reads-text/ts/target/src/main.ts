import { run } from "./lib.js";

// Each glob reads the files it matches as text or as their URL, never as modules.
const sources = import.meta.glob("./pages/*.ts", { query: "?raw", import: "default", eager: true });
const links = import.meta.glob("./links/*.ts", { as: "url" });
console.log(run(), sources, links);
