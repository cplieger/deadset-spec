// deadset:ignore DS1401 -- The next release reads note.
export function held(note: string): number {
  return 1;
}

export class Store {
  private n = 0;
  mark(committed: number): number { // deadset:ignore DS1401 -- An interface of the next release fixes the arity.
    return this.n;
  }
}

export function entry(key: string): number {
  return 2;
}

// deadset:ignore DS1401 -- Both are reserved for the decoder.
export function both(first: string, second: string): number {
  return 3;
}

export function wrapped(
  first: string,
  second: string, // deadset:ignore DS1401 -- Both are reserved for the decoder.
): number {
  return 4;
}

// deadset:ignore DS1401 -- Kept for a parameter that is read.
export function reads(a: number, b: number): number {
  return a + b;
}
