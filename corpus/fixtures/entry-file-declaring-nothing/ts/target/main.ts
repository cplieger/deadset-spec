// The entry file declares nothing: it runs one statement when it is loaded.
if (Date.now() < 0) {
  throw new Error("the clock reads before the epoch");
}
