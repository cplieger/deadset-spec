import { Scan } from "./scan.js";

const members: Record<string, number> = {};
new Scan(members).add("alpha");
if (Object.keys(members).length === 0) {
  throw new Error("nothing was stored");
}
