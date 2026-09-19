import { Counter, fresh, kept } from "./catalog.js";

// The entry file references one deprecated function, one that is not deprecated,
// and one member of the class.
const counter = new Counter(1);
if (kept() + fresh() + counter.live === 0) {
  throw new Error("the catalog returned zero");
}
