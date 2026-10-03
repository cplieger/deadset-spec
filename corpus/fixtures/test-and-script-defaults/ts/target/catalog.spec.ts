import { inSpec } from "./catalog.js";

if (inSpec() !== 1) {
  throw new Error("inSpec() is not 1");
}
