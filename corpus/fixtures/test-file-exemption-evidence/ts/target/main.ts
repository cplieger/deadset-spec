import { Draft, Sample } from "./record.js";

if (new Sample().id + new Draft().id === 0) {
  throw new Error("both identifiers are zero");
}
