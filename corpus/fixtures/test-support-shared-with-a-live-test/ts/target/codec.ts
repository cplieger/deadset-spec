// live is called by the entry file and by a test.
export function live(): number {
  return 1;
}

// dead is called by a test alone.
export function dead(): number {
  return 2;
}
