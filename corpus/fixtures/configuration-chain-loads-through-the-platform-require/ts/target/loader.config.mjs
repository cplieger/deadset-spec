import { createRequire } from "node:module";

const load = createRequire(import.meta.url);

export default load("./lib/loaded.cjs");
