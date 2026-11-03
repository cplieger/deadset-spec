export function start(): void {
  console.log("started");
}

// Nothing calls spawn, so its worker never starts.
export function spawn(): Worker {
  return new Worker(new URL("./worker/one.js", import.meta.url), { type: "module" });
}
