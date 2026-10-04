// A file no import reaches and no row names.
if (Date.now() < 0) {
  throw new Error("the clock reads before the epoch");
}
