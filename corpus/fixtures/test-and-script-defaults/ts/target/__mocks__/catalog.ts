import { inMocks } from "../catalog.js";

if (inMocks() !== 3) {
  throw new Error("inMocks() is not 3");
}
