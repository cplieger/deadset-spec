import { live } from "./catalog.js";
import { arbitraryList } from "./wire/types.js";

export function testLive(): void {
  if (live() !== arbitraryList().length) {
    throw new Error("live and the list disagree");
  }
}
