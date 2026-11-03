import { onlyTested } from "./lib.js";

if (onlyTested() !== 2) {
  throw new Error("onlyTested() is not 2");
}
