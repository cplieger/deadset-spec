import { platform } from "./main.js";
import { unused } from "./catalog.js";

// A test file under the default test-file pattern. A runner calls what it exports, so
// no declaration references the test.
export function testUnused(): void {
  if (unused() !== platform()) {
    throw new Error("unused changed");
  }
}
