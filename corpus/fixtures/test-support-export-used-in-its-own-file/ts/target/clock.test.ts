import { fakeNow } from "./support/setup.js";

// A test file under the default test-file pattern.
export function testClock(): void {
  if (fakeNow < 0) {
    throw new Error("no clock");
  }
}
