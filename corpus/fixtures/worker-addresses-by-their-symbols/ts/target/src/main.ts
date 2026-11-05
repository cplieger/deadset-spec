import { Worker as Thread } from "node:worker_threads";
import * as threads from "worker_threads";
import { own } from "./own.js";
import "./legacy.cjs";

// Node reads a thread's path against the working directory, the package's own.
new Thread("./dist/pool/emitted.js");
new threads.Worker("./src/pool/direct.ts");
new Thread("./beside.js");
new Thread("./src/pool/evaluated.ts", { eval: true });

// A browser reads a worker's address against the calling file.
new Worker("./pool/web.ts");
new Worker(new URL("pool/located.ts", import.meta.url), { type: "module" });
new globalThis.SharedWorker("shared.ts");
void globalThis.navigator.serviceWorker.register("./registered.ts");

// Nothing calls idle, so its thread never starts.
function idle(): void {
  new Thread("./src/pool/idle.ts");
}

void own;
