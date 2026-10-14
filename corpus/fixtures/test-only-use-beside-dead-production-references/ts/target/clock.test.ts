import { fixedNow, now } from "./clock.js";

// The test reaches live code, so it is no test of dead code.
export function testClock(): boolean {
  return now() > fixedNow;
}
