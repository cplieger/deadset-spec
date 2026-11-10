import { describe, Mode, Tier, tierOf } from "./levels.js";

// The entry file converts a number to a Tier, names the first and last Tier and
// the first Mode, and no other member.
if (
  tierOf(Math.floor(Math.random() * 3)) === Tier.Low &&
  tierOf(0) !== Tier.High &&
  describe(Mode.Read) === ""
) {
  throw new Error("the levels returned nothing");
}
