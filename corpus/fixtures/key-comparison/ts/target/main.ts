import { selected, type Frame } from "./keys.js";

export function deliver(evt: Frame): string {
  if (evt.type === "changed") {
    return "refetch";
  }
  switch (evt.type) {
    case "switched":
      return "switch";
  }
  return selected === "imported" ? "imported" : "dispatch";
}
