// Node's worker threads, as @types/node declares them.
declare module "node:worker_threads" {
  export class Worker {
    constructor(filename: string | URL, options?: { readonly eval?: boolean });
  }
}

declare module "worker_threads" {
  export * from "node:worker_threads";
}
