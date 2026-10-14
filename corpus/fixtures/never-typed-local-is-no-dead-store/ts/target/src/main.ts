type Key = "a" | "b";

function assigned(key: Key): number {
  let check: never;
  switch (key) {
    case "a":
      return 1;
    case "b":
      return 2;
    default:
      check = key;
      return 0;
  }
}

function initialized(key: Key): number {
  if (key === "a" || key === "b") {
    return 1;
  }
  const unreached: never = key;
  const other: string = key;
  return 0;
}

console.log(assigned("a"), initialized("b"));
