import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { solidStart } = require("@solidjs/start/config");

export default { plugins: [solidStart({ appRoot: "./web" })] };
