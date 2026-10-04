import { kept } from "./internal.js";

// published is called by a test file alone.
export function published(): number {
  return 1;
}

// live is called by nothing in the library, and it calls the internal module.
export function live(): number {
  return kept();
}
