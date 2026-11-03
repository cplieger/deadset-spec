export function start(): void {
  console.log("started");
}

// Nothing calls the functions below, so none of their loads runs.
export function loadLiteral(): Promise<unknown> {
  return import("./literal/one.js");
}

export function loadPlugin(name: string): Promise<unknown> {
  return import(`./plugins/${name}.js`);
}

export function loadEager(): Record<string, unknown> {
  return import.meta.glob("./eager/*.ts", { eager: true });
}

export function workerURL(): URL {
  return new URL("./worker/one.js", import.meta.url);
}
