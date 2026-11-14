import type { NoteA } from "./gen/v1/a.js";
import { NoteB } from "./encode/v1/b.js";
import type { NoteC } from "./bare/v1/c.js";

const notes: [NoteA, NoteC] = [{ text: "a" }, { text: "c" }];
console.log(notes, NoteB.encode({ text: "b" }).finish().length);
