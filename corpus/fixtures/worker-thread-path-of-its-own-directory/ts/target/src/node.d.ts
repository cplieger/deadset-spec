// Node's globals and path module, as @types/node declares them.
declare const __dirname: string;

interface ImportMeta {
  readonly dirname: string;
}

declare module "node:path" {
  export function join(...parts: string[]): string;
  export function resolve(...parts: string[]): string;
}
