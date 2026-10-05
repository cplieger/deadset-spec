import { live, seed } from "../catalog.js";
import { label } from "./service.js";

// open is called by the first helper file alone, and calls a live function beside the one only it calls.
export function open(): number {
  return seed() + live() + label().length;
}

// unused is called by nothing.
export function unused(): number {
  return 5;
}
