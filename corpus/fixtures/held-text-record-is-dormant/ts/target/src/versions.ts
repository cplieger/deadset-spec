let cached = "";
export function current(): string {
  return cached;
}
// deadset:ignore DS1102 -- test seam: tests reset the cache between cases
export function _resetForTest(): void {
  cached = "";
}
