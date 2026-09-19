import { deadOne, deadTwo, live } from "./catalog.js";

// The test file the default test-file pattern classifies as a test. The runner
// calls each exported test, so neither is referenced from a declaration.

export function testDeadOnly(): void {
  assertSum(deadOne() + deadTwo(), 3);
}

export function testMixed(): void {
  assertSum(deadOne() + live(), 4);
}

// assertSum is a helper the two tests share, declared in a test file and
// referenced from a test file alone.
function assertSum(got: number, want: number): void {
  if (got !== want) {
    throw new Error(`the sum of the declarations is ${got}, want ${want}`);
  }
}
