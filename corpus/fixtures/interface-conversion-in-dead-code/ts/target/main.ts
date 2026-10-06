import { Plain, use } from "./store.js";

if (use(new Plain()) !== "x") {
  throw new Error("the plain store changed the name");
}
