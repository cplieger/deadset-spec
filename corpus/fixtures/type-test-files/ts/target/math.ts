// add is called by the entry file.
export function add(a: number, b: number): number {
  return a + b;
}

// subtract is called by a type-test file alone.
export function subtract(a: number, b: number): number {
  return a - b;
}

// multiply is called by a file of a type-test directory alone.
export function multiply(a: number, b: number): number {
  return a * b;
}

// divide is called by a file with the .spec-d qualifier alone.
export function divide(a: number, b: number): number {
  return a / b;
}
