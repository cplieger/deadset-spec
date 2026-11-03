// The platform's URL class, as @types/node declares it.
declare module "url" {
  export class URL {
    constructor(input: string, base?: string);
    readonly href: string;
  }
}

declare module "node:url" {
  export * from "url";
}
