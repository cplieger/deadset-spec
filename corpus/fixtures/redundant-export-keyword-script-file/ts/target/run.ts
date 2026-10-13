// step is used only in this file.
export function step(): number {
  return 1;
}

if (step() !== 1) {
  throw new Error("step is not 1");
}
