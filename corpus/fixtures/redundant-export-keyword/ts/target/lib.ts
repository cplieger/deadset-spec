// The export the consumer package references, so the keyword carries a use
// outside the module that declares it.
export function published(): string {
  return "published";
}

// The export every reference of which is inside the module that declares it, so
// the keyword can go and the declaration stays.
export function local(): string {
  return "local";
}

// The reference to the export above, from the module that declares it, and an
// export the consumer references in its own right.
export function caller(): string {
  return local();
}
