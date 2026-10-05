import { open } from "./store.js";

// newService is called by a test.
export function newService(): number {
  return open() + 1;
}

// label is called by the second helper file alone, which this file imports.
export function label(): string {
  return "service";
}
