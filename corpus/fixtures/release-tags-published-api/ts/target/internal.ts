// The entry point's helper, and declarations the manifest does not export.

/** Formats a value for the entry point. */
export function helper(): string {
  return "helper";
}

/**
 * Builds a widget for embedders.
 * @public
 */
export function tagged(): number {
  return 1;
}

/** @beta */
export class Preview {
  /** Renders the preview. */
  render(): string {
    return "preview";
  }
}

/** @alpha */
export const experimental = 3;

/** Not tagged, so it is the library's internals. */
export function untagged(): number {
  return 2;
}

/** @internal */
export function hidden(): number {
  return 4;
}
