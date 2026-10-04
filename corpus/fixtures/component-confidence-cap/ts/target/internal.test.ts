import { internal } from "./internal.js";

export function testInternal(): void {
  if (internal() !== 2) {
    throw new Error("internal() is not 2");
  }
}
