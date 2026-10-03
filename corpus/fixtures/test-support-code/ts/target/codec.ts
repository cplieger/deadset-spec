// live is called by the entry file.
export function live(): number {
  return 1;
}

// encode is called by test-support code alone.
export function encode(): number {
  return 2;
}
