import type { Handle, Ledger, Note } from "./handles.js";
import { ledgerOf, note, open } from "./handles.js";

const handle: Handle<"alpha"> = open(1);
const read: Note = note(2);
const ledger: Ledger<string> = ledgerOf(["a", "b"]);

if (handle.id + read.id + ledger.total === 0) {
  throw new Error("every identifier is zero");
}
