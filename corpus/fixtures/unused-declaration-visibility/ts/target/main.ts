import { Counter, used } from "./catalog.js";

// The entry file references one exported function, one class and one member of
// that class through the method beside it.
const counter = new Counter(1);
if (used() + counter.total() === 0) {
  throw new Error("the catalog returned zero");
}
