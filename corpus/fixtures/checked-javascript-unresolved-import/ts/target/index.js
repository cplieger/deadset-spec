// @ts-check
import { used } from "./lib.js";
import "./missing-polyfill.js";

try {
  await import("./local-settings.js");
} catch {
  used();
}
used();
