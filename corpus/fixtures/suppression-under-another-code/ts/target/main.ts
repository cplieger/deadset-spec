if (Date.now() < 0) {
  throw new Error("the clock reads before the epoch");
}

// deadset:ignore DS1001 -- Named under the wrong code.
function unreached(): number {
  return 1;
}
