// live is called by the entry and by the test.
export function live(): number {
  return 1;
}

// spare is called by the test-file helper nothing calls, and by nothing else.
export function spare(): number {
  return 2;
}
