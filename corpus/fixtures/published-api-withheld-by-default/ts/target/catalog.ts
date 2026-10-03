// Catalog is a class the published API names.
export class Catalog {
  // exposed is a public member of the published class that nothing calls.
  exposed(): number {
    return 1;
  }

  // hidden is a member only this class can name, and nothing does.
  private hidden(): number {
    return 2;
  }
}

// count is a published function nothing calls.
export function count(c: Catalog | undefined): number {
  return c === undefined ? 0 : 1;
}

// orphan is a function only this module can name, and nothing does.
function orphan(): number {
  return 3;
}
