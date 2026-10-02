// Called by the entry statement below and never names T.
function keep<
  T,
>(n: number): number {
  return n;
}

// Called by nothing and never names U.
function pick<
  U,
>(n: number): number {
  return n;
}

if (keep<number>(1) !== 1) {
  throw new Error("keep changed its argument");
}
