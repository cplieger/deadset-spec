interface Payload {
  open: string;
  close: string;
  idle: string;
}
type Kind = "open" | "close";
const payload: Payload = { open: "o", close: "c", idle: "i" };
export function payloadOf<K extends Kind>(m: Payload, k: K): string {
  return m[k];
}

interface Fields {
  title: string;
  body: string;
  extra: string;
}
const fields: Fields = { title: "t", body: "b", extra: "e" };
function at(key: "title" | "body"): string {
  [key] = ["body"];
  return fields[key];
}

interface Looped {
  left: string;
  right: string;
  spare: string;
}
const looped: Looped = { left: "l", right: "r", spare: "s" };
function pick(key: "left" | "right"): string {
  for (key of ["right"] as const) {
    break;
  }
  return looped[key];
}
export const shown = [payloadOf(payload, "open"), at("title"), pick("left")];
