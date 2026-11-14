// used is the export the binding of the awaited namespace is read for.
export function used(): number {
  return 1;
}

// spare is an export the binding is never read for.
export function spare(): number {
  return 2;
}
