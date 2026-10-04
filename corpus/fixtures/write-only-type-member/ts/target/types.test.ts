import { entryOf } from "./types.js";

export function testTag(): void {
  if (entryOf("a").tag !== "t") {
    throw new Error("the tag is not t");
  }
}
