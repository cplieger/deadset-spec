interface Partial {
  uploaded: string[];
  skipped: string[];
}

export const make = (u: string[]): unknown => ({ uploaded: u, skipped: u }) satisfies Partial;

export const read = (c: unknown): unknown =>
  typeof c === "object" && c !== null && "uploaded" in c ? c.uploaded : [];
