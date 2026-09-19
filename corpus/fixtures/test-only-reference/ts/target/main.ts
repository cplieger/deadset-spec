import { production } from "./catalog.js";

// The entry file references one of the two declarations, which is what makes the
// other one's references test references alone.
if (production() === 0) {
  throw new Error("the catalog returned zero");
}
