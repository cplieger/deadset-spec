import { published } from "./catalog.js";

export function testPublished(): void {
  if (published() !== 1) {
    throw new Error("published() is not 1");
  }
}
