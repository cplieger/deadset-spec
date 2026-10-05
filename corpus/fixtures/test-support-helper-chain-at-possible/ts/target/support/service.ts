import { open } from "./store.js";

// newService is called by a test.
export function newService(): number {
  return open() + 1;
}
