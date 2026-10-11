import { dead, live } from "./codec.js";
import { helper } from "./support/helpers.js";

export function testLive(): void {
  if (live() + helper() !== 4) {
    throw new Error("live and helper do not add up to 4");
  }
}

export function testDead(): void {
  if (dead() + helper() !== 5) {
    throw new Error("dead and helper do not add up to 5");
  }
}
