import { live } from "./codec.js";
import { fixture } from "./support/fixtures.js";

if (fixture() !== live() + 2) {
  throw new Error("fixture and live disagree");
}
