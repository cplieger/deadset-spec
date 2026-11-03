import { URL } from "node:url";

export function start(): void {
  console.log("started", new URL(import.meta.url).pathname);
}

// Nothing calls spawn, so its worker never starts.
export function spawn(): Worker {
  return new Worker(new URL("./worker/one.js", import.meta.url), { type: "module" });
}
