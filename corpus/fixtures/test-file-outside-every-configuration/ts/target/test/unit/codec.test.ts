import { forTests, live } from "../../src/codec.js";

export function testForTests(): void {
  const unused = forTests();
  const value = forTests();
  if (value !== 2 || live() !== 1) {
    throw new Error("codec returns the wrong values");
  }
}
