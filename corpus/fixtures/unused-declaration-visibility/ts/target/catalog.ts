// The module file the entry imports. It is named by no manifest entry point, so
// its exports are not roots and the analysis judges them by their references.

// used is exported and the entry file references it.
export function used(): number {
  return usedHelper();
}

// resolve is exported and nothing references it.
export function resolve(): number {
  return 2;
}

// usedHelper is not exported and used references it.
function usedHelper(): number {
  return 3;
}

// helper is not exported and nothing references it.
function helper(): number {
  return 4;
}

// recurse calls itself and nothing else calls it, so it carries one reference and
// no path from a root reaches it.
export function recurse(n: number): number {
  return n <= 0 ? 0 : recurse(n - 1);
}

// Counter carries one member the entry reads through the method beside it and one
// member nothing names.
export class Counter {
  live: number;

  private stale?: number;

  constructor(live: number) {
    this.live = live;
  }

  total(): number {
    return this.live;
  }
}
