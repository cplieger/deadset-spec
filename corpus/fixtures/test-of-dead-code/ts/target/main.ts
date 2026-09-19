import { live } from "./catalog.js";

// The entry file references the one declaration that is live, which is what makes
// the test referencing only the other two a test of dead code.
if (live() === 0) {
  throw new Error("the catalog returned zero");
}
