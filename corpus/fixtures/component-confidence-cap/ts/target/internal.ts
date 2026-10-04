// A module the published module imports and no manifest export reaches.

// internal is called by a test file alone.
export function internal(): number {
  return 2;
}

// kept is called by the published module.
export function kept(): number {
  return 3;
}
