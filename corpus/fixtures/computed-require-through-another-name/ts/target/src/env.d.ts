// Node's createRequire, as @types/node declares it.
declare module "node:module" {
  export function createRequire(path: string): (id: string) => unknown;
}
