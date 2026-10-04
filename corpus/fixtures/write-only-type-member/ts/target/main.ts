import type { Entry, Row } from "./types.js";
import { entryOf, rowsOf } from "./types.js";

const entry: Entry = entryOf("k");
const rows: Row[] = rowsOf([1]);

if (entry.key.length + rows.length === 0) {
  throw new Error("nothing was read");
}
for (const one of rows) {
  if (one.id < 0) {
    throw new Error("a negative identifier");
  }
}
