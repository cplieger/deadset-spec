// a is read, b is never read.
import {
  a,
  b,
} from "./lib.js";
// Shape is read by a type position.
import type { Shape } from "./lib.js";
// whole is a namespace binding nothing reads.
import * as whole from "./lib.js";

const shaped: Shape = { n: a };
if (shaped.n !== 1) {
  throw new Error("shaped.n is not 1");
}
