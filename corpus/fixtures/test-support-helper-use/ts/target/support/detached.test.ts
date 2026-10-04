import { detached } from "./detached.js";

export function testDetached(): void {
  if (detached() !== 7) {
    throw new Error("detached() is not 7");
  }
}
