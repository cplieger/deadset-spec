// A file no import reaches, no row names and no entry file names.
if (Date.now() < 0) {
  throw new Error("the clock reads before the epoch");
}
