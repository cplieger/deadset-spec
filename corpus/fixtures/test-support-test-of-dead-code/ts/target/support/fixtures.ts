import { decode } from "../codec.js";

// fixture is called by a test and by check.
export function fixture(): number {
  return 4;
}

// check calls fixture and a production function only it calls.
export function check(): number {
  return decode() + fixture();
}
