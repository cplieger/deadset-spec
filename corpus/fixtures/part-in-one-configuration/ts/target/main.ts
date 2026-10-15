export function halt(): number {
  stop();
  console.log("after");
  return 1;
}

export function done(): number {
  return 2;
  console.log("never");
}
