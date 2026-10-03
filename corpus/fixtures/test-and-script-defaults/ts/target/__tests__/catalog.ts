import { inTests } from "../catalog.js";

if (inTests() !== 2) {
  throw new Error("inTests() is not 2");
}
