// live is called by the entry file.
export function live(): number {
  return 1;
}

// encode is called by a test-file helper that nothing calls.
export function encode(): number {
  return 2;
}

// decode is called by a test whose every reference is dead.
export function decode(): number {
  return 3;
}

// parse is called by a test-file helper that a test calls.
export function parse(): number {
  return 5;
}

// format is called by a test-file helper that also calls a live declaration.
export function format(): number {
  return 6;
}
