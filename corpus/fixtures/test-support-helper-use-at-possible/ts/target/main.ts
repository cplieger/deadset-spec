import { live } from "./catalog.js";

if (live() !== 1) {
  throw new Error("live() is not 1");
}
