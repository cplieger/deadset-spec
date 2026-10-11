import { forTests, live } from "../src/codec.js";

export function testCodec(): void {
  const unused = forTests();
  if (forTests() !== 2 || live() !== 1) {
    throw new Error("codec returns the wrong values");
  }
}
