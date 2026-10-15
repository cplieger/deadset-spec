if (Date.now() < 0) {
  throw new Error("the clock reads before the epoch");
}

function dead(): number {
  return helper(1);
}

// deadset:ignore DS1401 -- Kept for a parameter that is read.
function helper(v: number): number {
  return v + 1;
}

// deadset:ignore DS1002 -- Kept for the next release.
function alsoDead(): number {
  return 2;
}
