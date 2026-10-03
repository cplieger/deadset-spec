import { used } from "@example/lib";

if (used() !== 1) {
  throw new Error("used() is not 1");
}
