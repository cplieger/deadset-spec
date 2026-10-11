import { createBus } from "@example/bus";

interface Fields {
  title: string;
  body: string;
  extra: string;
}
const fields: Fields = { title: "t", body: "b", extra: "e" };
const at = (key: keyof Fields) => fields[key];
export const shown = [at("title"), at("body")];

interface Picked {
  left: string;
  right: string;
}
const picked: Picked = { left: "l", right: "r" };
const pick = (key: keyof Picked) => picked[key];
function register(fn: (key: keyof Picked) => string): string {
  return fn("left");
}
export const registered = register(pick);

interface Draft {
  a: number;
  b: number;
}
const draft: Draft = { a: 0, b: 0 };
const keys: ("a" | "b")[] = ["a", "b"];
for (const k of keys) {
  draft[k] = 1;
}

interface Other {
  x: number;
  y: number;
  z: number;
}
const other: Other = { x: 1, y: 2, z: 3 };
function choose(): "x" | "y" {
  return Date.now() > 0 ? "x" : "y";
}
export function readOther(): number {
  const k: "x" | "y" = choose();
  return other[k];
}

interface Signals {
  closed: string;
  opened: string;
}
const bus = createBus<Signals>();
bus.clear("closed");

interface Doc {
  title: string;
  author: string;
}
interface Reader {
  get(key: keyof Doc): string;
}
const doc: Doc = { title: "t", author: "a" };
class DocReader implements Reader {
  get(key: keyof Doc): string {
    return doc[key];
  }
}
const reader: Reader = new DocReader();
export const docTitle = reader.get("title");

interface Modes {
  light: string;
  dark: string;
}
const modes: Modes = { light: "l", dark: "d" };
function mode(key: keyof Modes = "light"): string {
  return modes[key];
}
export const current = mode();
