import { published, caller } from "../target/lib.js";

// The references that keep two of the target's exports live.
if (published() === "" || caller() === "") {
  throw new Error("the target returned an empty string");
}
