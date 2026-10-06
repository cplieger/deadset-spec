import { Box, total } from "./shape.js";

if (total(new Box()) !== 2) {
  throw new Error("the box has the wrong size");
}
