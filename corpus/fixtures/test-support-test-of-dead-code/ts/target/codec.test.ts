import { encode } from "./codec.js";
import { check, fixture } from "./support/fixtures.js";

export function testEncode(): void {
  if (encode() + fixture() !== 6) {
    throw new Error("encode and fixture do not add up to 6");
  }
}

export function testDecode(): void {
  if (check() !== 7) {
    throw new Error("check is not 7");
  }
}
