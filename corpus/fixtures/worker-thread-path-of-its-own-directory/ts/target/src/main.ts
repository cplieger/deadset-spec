import { Worker } from "node:worker_threads";
import * as path from "node:path";
import { resolve } from "node:path";

declare const name: string;

// Node reads a path built from the file's own directory against that directory.
new Worker(path.join(__dirname, "pool", "joined.ts"));
new Worker(resolve(import.meta.dirname, "./pool/resolved.ts"));
new Worker(path.join(__dirname, "pool", name));

// Nothing calls idle, so its thread never starts.
function idle(): void {
  new Worker(path.join(__dirname, "pool/idle.ts"));
}
