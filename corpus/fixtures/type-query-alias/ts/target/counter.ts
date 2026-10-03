// useCounter is used through the global that aliases it.
export function useCounter(): number {
  return 1;
}

// resetCounter is aliased by a global nothing uses.
export function resetCounter(): void {}

// unusedCounter is aliased by nothing.
export function unusedCounter(): number {
  return 0;
}
