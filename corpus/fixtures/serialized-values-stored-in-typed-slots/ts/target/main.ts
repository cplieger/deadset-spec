interface Base {
  id: string;
}
interface Derived extends Base {
  extra: string;
}
interface Other extends Base {
  spare: string;
}
interface Box {
  item: Base;
}
const derived: Derived = { id: "d", extra: "e" };
const box: Box = { item: { id: "b" } };
box.item = derived;
const other: Other = { id: "o", spare: "s" };
const base: Base = other;

interface Aliased {
  note: string;
}
type Slot = unknown;
const aliased: Aliased = { note: "n" };
let slot: Slot;
slot = aliased;

interface Listed {
  note: string;
}
const listed: Listed = { note: "n" };
let first: unknown;
[first] = [listed];

interface Picked {
  note: string;
}
interface Unsent {
  note: string;
}
const source: { value: Picked; skipped: Unsent } = { value: { note: "p" }, skipped: { note: "u" } };
let picked: unknown;
({ value: picked } = source);

export const body = [
  JSON.stringify(box),
  JSON.stringify(base),
  JSON.stringify(slot),
  JSON.stringify(first),
  JSON.stringify(picked),
];
