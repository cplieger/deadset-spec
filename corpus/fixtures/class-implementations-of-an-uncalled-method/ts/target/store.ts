// Store keeps values by key.
interface Store {
  get(key: string): number;
  reset(): void;
}

// Memory keeps values in a map.
export class Memory implements Store {
  readonly values = new Map<string, number>();

  get(key: string): number {
    return this.values.get(key) ?? 0;
  }

  // reset clears the map. Nothing calls it, through Store or directly.
  reset(): void {
    this.values.clear();
  }
}

// Disk keeps values in a file.
export class Disk implements Store {
  constructor(readonly path: string) {}

  get(key: string): number {
    return key.length + this.path.length;
  }

  // reset is called directly by the entry.
  reset(): void {
    if (this.path.length === 0) {
      throw new Error("the disk has no path");
    }
  }
}

// read reads one value through the interface.
export function read(store: Store, key: string): number {
  return store.get(key);
}

// Clearer clears what it holds.
interface Clearer {
  reset(): void;
}

// Cache keeps values in a map. Its reset implements both Store.reset, which
// nothing calls, and Clearer.reset, which clear calls.
export class Cache implements Store, Clearer {
  readonly values = new Map<string, number>();

  get(key: string): number {
    return this.values.get(key) ?? 0;
  }

  reset(): void {
    this.values.clear();
  }
}

// clear clears a value through the interface.
export function clear(clearer: Clearer): void {
  clearer.reset();
}
