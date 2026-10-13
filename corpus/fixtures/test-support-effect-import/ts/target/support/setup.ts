import "vitest";

// A test imports this file for its effects alone.
function seed(): number {
  return 2;
}

if (seed() !== 2) {
  throw new Error("seed is not 2");
}
