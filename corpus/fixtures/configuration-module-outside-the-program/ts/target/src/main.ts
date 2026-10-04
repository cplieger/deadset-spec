// The entry file imports nothing.
if (Date.now() < 0) {
  throw new Error("the clock reads before the epoch");
}
