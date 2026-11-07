let cached = "";

export function heldValue(): string {
  return cached;
}

// deadset:ignore DS1102 -- test seam: tests reset the cache between cases
export function _resetHeldForTest(): void {
  cached = "";
}
