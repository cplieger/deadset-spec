// assist is called by the published entry.
export function assist(): number {
  return 1;
}

// unusedInternal is exported by a file no manifest entry reaches.
export function unusedInternal(): number {
  return 3;
}
