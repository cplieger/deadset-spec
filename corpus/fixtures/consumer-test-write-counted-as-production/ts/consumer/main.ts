import { defaults } from "../target/options.js";

// The consumer checks that the target returns options.
if (defaults() === undefined) {
  throw new Error("no options");
}
