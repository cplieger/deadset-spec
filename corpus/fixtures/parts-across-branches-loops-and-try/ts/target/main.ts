// branched overwrites its first write on both branches before the read.
export function branched(flag: boolean): number {
  let value = 1;
  if (flag) {
    value = 2;
  } else {
    value = 3;
  }
  return value;
}

// partial keeps its first write, which the branch with no write reads.
export function partial(flag: boolean): number {
  let value = 1;
  if (flag) {
    value = 2;
  }
  return value;
}

// last overwrites each iteration's write after the loop.
export function last(items: readonly number[]): number {
  let value = 0;
  for (const item of items) {
    value = item;
  }
  value = -1;
  return value;
}

// named falls from one clause into the next clause's write.
export function named(kind: number): string {
  let label = "";
  switch (kind) {
    case 1:
      label = "one";
    case 2:
      label = "two";
      break;
    default:
      label = "other";
  }
  return label;
}

// finished leaves through its try block, so the line after the try never runs.
export function finished(): number {
  try {
    return 1;
  } finally {
    console.log("cleanup");
  }
  console.log("after the try");
}

// caught falls out of its catch block, so its last statement runs.
export function caught(): number {
  try {
    return 1;
  } catch {
    console.log("failed");
  }
  return 2;
}
