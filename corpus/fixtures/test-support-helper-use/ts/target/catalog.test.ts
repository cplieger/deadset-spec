import { detached, drain } from "./support/detached.js";

// fixture is a test-file helper that calls test-support code alone.
function fixture(): number {
  return detached();
}

export function testDrain(): void {
  let n = 0;
  drain({
    put(v: number): void {
      n += v;
    },
  });
  if (n + fixture() !== 8) {
    throw new Error(`drained ${n}`);
  }
}
