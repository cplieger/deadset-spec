// apply calls the function it is handed with two arguments.
export function apply(fn: (a: number, b: number) => number): number {
  return fn(1, 2);
}

// infer's inference may read the annotation of the parameter its type parameter types.
export function infer<T>(fn: (value: T) => number): T[] {
  fn(undefined as T);
  return [];
}
