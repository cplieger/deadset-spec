// live is called by the entry file.
export function live(): number {
  return 1;
}

// encode is called by a test alone.
export function encode(): number {
  return 2;
}

// decode is called by test-support code alone.
export function decode(): number {
  return 3;
}
