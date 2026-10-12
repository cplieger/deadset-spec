// The target adds a member to the global URL interface, which stays the global URL.
declare global {
  interface URL {
    readonly extra?: number;
  }
}

const worker = new URL("./worker.ts", import.meta.url);
console.log(worker.href, worker.extra);
