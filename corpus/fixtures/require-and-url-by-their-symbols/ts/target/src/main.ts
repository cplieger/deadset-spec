import { createRequire } from "node:module";
import * as url from "node:url";
import { URL as NodeURL } from "node:url";

const load = createRequire(import.meta.url);
const again = load;

load("./loaded/created.js");
(load)("./loaded/parenthesized.js");
again("./loaded/aliased.js");

export const workers = [
  new (URL)("./loaded/parenthesized-url.js", import.meta.url),
  new NodeURL("./loaded/renamed-url.js", import.meta.url),
  new url.URL("./loaded/namespace-url.js", import.meta.url),
  new globalThis.URL("./loaded/global-url.js", import.meta.url),
];

// The target's own function, whose body loads nothing.
function fetchText(path: string): void {
  console.log(path);
}

fetchText("./loaded/own.js");
