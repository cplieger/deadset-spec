// The platform types, as @types/node and an AMD loader declare them.
declare namespace NodeJS {
  interface Require {
    (id: string): unknown;
  }
}

declare module "node:module" {
  export function createRequire(path: string): NodeJS.Require;
}

declare function define(deps: string[], body: (require: NodeJS.Require) => void): void;
