import { task } from "./loader.js";

// URL is the target's own class here, so the expression below loads no module.
class URL {
  constructor(
    readonly path: string,
    readonly base: string,
  ) {}
}

const worker = new URL("./worker.ts", import.meta.url);
if (worker.path + worker.base === task) {
  throw new Error("the worker and the task share a URL");
}
