// Store is the interface production code reads through.
export interface Store {
  get(key: string): number;
}

// Disk is the implementation the entry file passes.
export class Disk implements Store {
  get(key: string): number {
    return key.length;
  }
}

// total reads through the interface.
export function total(s: Store): number {
  return s.get("a") + 1;
}

// seed is called by the fake's method alone.
export function seed(): number {
  return 2;
}
