import "./support/setup.js";
import { live } from "./catalog.js";

export function testLive(): void {
  if (live() !== 1) {
    throw new Error("live is not 1");
  }
}
