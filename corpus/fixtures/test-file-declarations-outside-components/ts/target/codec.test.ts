import { decode, encode, format, live, parse } from "./codec.js";

// A test file under the default test-file pattern. A runner evaluates it as a
// module and calls each exported test by name.

// encodeAll is a test-file helper that nothing calls.
function encodeAll(): number {
  return encode();
}

export function testDecode(): void {
  if (decode() !== 3) {
    throw new Error("decode returned the wrong value");
  }
}

// mustParse is a test-file helper a test calls.
function mustParse(): number {
  return parse();
}

export function testRoundTrip(): void {
  if (mustParse() !== live() + 4) {
    throw new Error("parse and live disagree");
  }
}

// formatBoth is a test-file helper that nothing calls.
function formatBoth(): number {
  return format() + live();
}
