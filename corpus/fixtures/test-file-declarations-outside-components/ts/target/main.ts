import { live } from "./codec.js";

// The entry file calls the one function of the module that is live.
if (live() === 0) {
  throw new Error("the codec returned zero");
}
