// The entry starts a worker from a module URL resolved against its own URL.
const worker = new URL("./worker.ts", import.meta.url);
export const started = worker.href;
