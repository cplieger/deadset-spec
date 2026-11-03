import { createRequire } from "node:module";
import "./amd.js";

const require = createRequire(import.meta.url);
const plugin = String(Date.now());
console.log(require("./plugins/" + plugin));
