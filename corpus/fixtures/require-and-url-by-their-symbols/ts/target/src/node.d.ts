// The platform's require and URL, as Node's type declarations write them.
declare namespace NodeJS {
  interface Require {
    (id: string): unknown;
  }
}

declare module "node:module" {
  export function createRequire(path: string): NodeJS.Require;
}

declare module "node:url" {
  export const URL: typeof globalThis.URL;
}
