// Entry is the shape the catalog builds.
interface Entry {
  readonly name: string;
  readonly size: number;
  readonly note: string;
}

// Named is the shape a comparator reads.
interface Named {
  readonly name: string;
}

// Sized is the shape a total reads.
interface Sized {
  readonly size: number;
}

function byName(left: Named, right: Named): number {
  return left.name.localeCompare(right.name);
}

function total(items: readonly Sized[]): number {
  let sum = 0;
  for (const item of items) {
    sum += item.size;
  }
  return sum;
}

function build(): Entry[] {
  return [
    { name: "b", size: 2, note: "" },
    { name: "a", size: 1, note: "" },
  ];
}

const entries = build();

// The entry sorts the entries with a comparator and totals them.
export const report = [entries.toSorted(byName).length, total(entries)];
