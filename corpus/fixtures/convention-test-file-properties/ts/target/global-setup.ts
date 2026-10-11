import { warm } from "./src/support.ts";

declare module "vitest" {
  export interface ProvidedContext {
    port: number;
  }
}

export function setup(): void {
  warm();
}
