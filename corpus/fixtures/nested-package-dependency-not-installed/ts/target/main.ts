import { pad } from "./tool/pad.js";

if (pad("x") === "") {
  throw new Error("pad is empty");
}
