// Tier is produced by a type assertion from a number, so a value of it can arrive
// by value rather than by name.
export enum Tier {
  Low,
  Mid,
  High,
}

// tierOf converts a number to a Tier.
export function tierOf(n: number): Tier {
  return n as Tier;
}

// Mode is produced by no conversion.
export enum Mode {
  Read,
  Write,
}

// describe spells one mode.
export function describe(m: Mode): string {
  return m === Mode.Read ? "read" : "other";
}
