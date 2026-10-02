// The entry file runs one statement when it is loaded.
if (Date.now() < 0) {
  throw new Error("the clock reads before the epoch");
}

// firstLine is called by nothing, and its body never reads chunk; the
// directive above it names the parameter's code alone.
// deadset:ignore DS1801 -- Kept to match the decoder signature the next release adds.
function firstLine(chunk: Uint8Array): number {
  return 1;
}

// wrapped is called by nothing, and its body never reads chunk; the directive
// inside the parameter list names the parameter's code alone.
function wrapped(
  // deadset:ignore DS1801 -- Kept to match the decoder signature the next release adds.
  chunk: Uint8Array,
): number {
  return 2;
}
