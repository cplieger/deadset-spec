import { onlyTested, production } from "./catalog.js";

// The test file the default test-file pattern classifies as a test, so every
// reference it makes is a test reference.
if (onlyTested() !== 1) {
  throw new Error("onlyTested() is not 1");
}
if (production() !== 2) {
  throw new Error("production() is not 2");
}
