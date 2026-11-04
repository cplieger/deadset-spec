import { URL } from "node:url";

const worker = new URL("./worker.ts", import.meta.url);
console.log(worker.href);
