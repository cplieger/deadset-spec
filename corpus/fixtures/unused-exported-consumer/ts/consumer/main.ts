import { usedByConsumer } from "../target/lib.js";

// The reference that keeps one of the target's exports live.
if (usedByConsumer() === "") {
  throw new Error("the target returned an empty string");
}
