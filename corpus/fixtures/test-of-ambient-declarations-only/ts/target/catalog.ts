// Production code imports this module for live() alone.
export function unused(): number {
  return 1;
}

// No production file calls this one either.
export function unusedToo(): number {
  return 0;
}

export function live(): number {
  return 2;
}
