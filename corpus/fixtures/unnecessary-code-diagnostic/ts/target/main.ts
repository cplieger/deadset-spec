// scaled names a label its body never reads, which the compiler options make an
// error.
function scaled(value: number, label: string): number {
  return value * 2;
}

if (scaled(1, "a") !== 2) {
  throw new Error("scaled() is not 2");
}
