// live is called by the entry file.
export function live(): number {
  return 1;
}

// forTests is called by the target's test file alone.
export function forTests(): number {
  return 2;
}
