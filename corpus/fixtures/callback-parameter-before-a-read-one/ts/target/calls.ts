// apply calls the function it is handed with two arguments.
export function apply(fn: (a: number, b: number) => number): number {
  return fn(1, 2);
}
