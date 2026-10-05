import { live } from "./codec.js";

// helper is a test-file helper a test calls, and it never reads its second parameter.
function helper(n: number, unread: number): number {
  return live(n);
}

// deadHelper is a test-file helper nothing calls, with the same shape.
function deadHelper(n: number, unread: number): number {
  return live(n);
}

export function testLive(): void {
  let count = 0;
  count = helper(1, 2);
  if (count !== 1) {
    throw new Error("live returned the wrong value");
  }
}
