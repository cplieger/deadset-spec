import { production } from "./catalog.js";

if (production() !== 4) {
  throw new Error("production() is not 4");
}
