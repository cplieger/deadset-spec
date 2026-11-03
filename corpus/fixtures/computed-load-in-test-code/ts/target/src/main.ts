import { production } from "./catalog.js";

if (production() === 0) {
  throw new Error("the catalog returned zero");
}
