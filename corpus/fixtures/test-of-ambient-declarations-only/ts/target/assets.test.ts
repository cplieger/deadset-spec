import { unused, unusedToo } from "./catalog.js";

// A test file under the default test-file pattern. A runner calls what it exports, so
// no declaration references either test.
export function testGlob(): void {
  if (Object.keys(import.meta.glob("./*.css")).length === 0) {
    throw new Error("no stylesheet");
  }
}

// This test references only a target function nothing else uses.
export function testDead(): void {
  if (unused() !== 1) {
    throw new Error("unused changed");
  }
}

// This test references a target function nothing else uses and the ambient member.
export function testBoth(): void {
  if (unusedToo() !== Object.keys(import.meta.glob("./*.css")).length) {
    throw new Error("unusedToo changed");
  }
}
