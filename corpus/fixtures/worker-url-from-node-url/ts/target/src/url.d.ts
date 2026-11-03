// The platform's URL class, which node:url exports as the global names it.
declare module "url" {
  export const URL: typeof globalThis.URL;
}

declare module "node:url" {
  export * from "url";
}
