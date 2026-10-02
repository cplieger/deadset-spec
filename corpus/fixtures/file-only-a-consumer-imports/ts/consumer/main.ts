import { greet } from "../target/extra.js";

// The consumer imports a file of the target that the target itself does not.
if (greet() === "") {
  throw new Error("the target returned an empty string");
}
