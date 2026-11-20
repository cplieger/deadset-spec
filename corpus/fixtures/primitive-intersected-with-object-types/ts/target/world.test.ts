import type { Model } from "./model.js";

export function facts(held: Model): unknown {
  return held.facts;
}
