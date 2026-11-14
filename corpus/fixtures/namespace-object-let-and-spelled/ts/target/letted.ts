// shownByLet is read through a let binding of the namespace object.
export function shownByLet(): number {
  return 1;
}

// spareByLet is read by no name; the let binding holds the object whole.
export function spareByLet(): number {
  return 2;
}
