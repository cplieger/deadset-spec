// The module file the entry imports. It is named by no manifest entry point, so
// its exports are not roots and the analysis judges them by their references. Each
// deprecation is the documentation tag the TypeScript tools recognize.

/**
 * Resolved an alias the way the first release did.
 *
 * @deprecated nothing references it; fresh does the same work.
 */
export function old(): number {
  return 1;
}

/**
 * Resolves an alias.
 *
 * @deprecated fresh does the same work, and the entry file still calls this one.
 */
export function kept(): number {
  return 2;
}

/**
 * Deprecated in a declaration that names two variables, so the tag covers both.
 *
 * @deprecated nothing reads either of them.
 */
export const stale = 3,
  staleToo = 4;

/** Resolves an alias. */
export function fresh(): number {
  return 5;
}

// Counter carries a deprecated member nothing reads beside a member the entry
// file reads.
export class Counter {
  live: number;

  /** @deprecated nothing reads it. */
  old?: number;

  constructor(live: number) {
    this.live = live;
  }
}
