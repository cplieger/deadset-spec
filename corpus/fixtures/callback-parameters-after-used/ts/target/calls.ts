// The two functions the entry passes its callbacks to. Each calls its callback
// with every argument the callback's parameter type declares.

export function apply(fn: (a: number, b: number, c: number) => number, n: number): number {
  return fn(n, n + 1, n + 2);
}

export function each(fn: (a: number, b: number) => number): number {
  return fn(1, 2);
}

// Pair is the value withPair and withScaled pass their callbacks.
interface Pair {
  readonly left: number;
  readonly right: number;
}

export function withPair(fn: (pair: Pair, scale: number) => number): number {
  return fn({ left: 1, right: 2 }, 3);
}

export function withScaled(fn: (scale: number, pair: Pair) => number): number {
  return fn(3, { left: 1, right: 2 });
}

// Box is the value withBox calls its callback on.
export interface Box {
  readonly n: number;
}

export function withBox(fn: (this: Box) => number): number {
  const box: Box = { n: 1 };
  return fn.call(box) + box.n;
}
