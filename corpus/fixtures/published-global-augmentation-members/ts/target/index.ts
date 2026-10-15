import type { Shape } from "./shape.ts";

declare global {
  interface Window {
    fromEntry?: unknown;
  }
}

export function area(shape: Shape): number {
  return shape.side;
}
