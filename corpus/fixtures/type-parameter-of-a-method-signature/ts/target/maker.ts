// Maker declares a method signature whose type parameter it names nowhere.
export interface Maker {
  build<T>(): string;
}

// make returns a Maker.
export function make(): Maker {
  return { build: () => "built" };
}

// shape is a function whose type parameter it names nowhere.
export function shape<U>(): string {
  return "shape";
}
