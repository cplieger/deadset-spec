import { Disk, seed, type Store } from "../store.js";

// Memory implements Store for tests.
export class Memory implements Store {
  // get is called through the interface alone, and calls a live method beside the function only it calls.
  get(key: string): number {
    return seed() + new Disk().get(key) + 4;
  }
}

// newMemory is called by a test.
export function newMemory(): Memory {
  return new Memory();
}

// unused is called by nothing.
export function unused(): number {
  return 3;
}
