import { describe, Mode, Tier, tierOf } from "./levels.js";

// The entry file converts a number to a Tier and names the first member of each
// enum and no other.
if (tierOf(Math.floor(Math.random() * 3)) === Tier.Low && describe(Mode.Read) === "") {
  throw new Error("the levels returned nothing");
}
