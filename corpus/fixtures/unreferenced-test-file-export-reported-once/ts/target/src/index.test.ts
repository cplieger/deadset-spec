import { api } from "./index.js";

// notCalled is exported by the test file and referenced by nothing.
export const notCalled = 1;

if (api() !== 1) {
  throw new Error("api answers 1");
}
