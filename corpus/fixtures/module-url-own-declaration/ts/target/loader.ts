// The loader starts a task from a module URL resolved against its own URL.
export const task = new URL("./task.ts", import.meta.url).href;
