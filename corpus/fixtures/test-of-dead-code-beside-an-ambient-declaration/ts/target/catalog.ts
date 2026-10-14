// Production code imports this module for live() alone.
export function unused(): number {
  return 1;
}

export function live(): number {
  return 2;
}
