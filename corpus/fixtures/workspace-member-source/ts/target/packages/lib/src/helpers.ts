// used is re-exported by the library's entry and called by the application.
export function used(): number {
  return 1;
}

// unused is re-exported by nothing and called by nothing.
export function unused(): number {
  return 2;
}
