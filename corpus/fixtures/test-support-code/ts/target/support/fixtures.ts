import { encode, live } from "../codec.js";

// fixture is called by a test, and calls a live function beside the one only it calls.
export function fixture(): number {
  return encode() + live();
}

// unused is called by nothing.
export function unused(): number {
  return 3;
}
