import { live, spare } from "./catalog.js";

// A runner loads the file and calls what it registers or exports.
export function testLive(): void {
  if (live() !== 1) {
    throw new Error("live is not 1");
  }
}

// unusedHelper is called by nothing.
function unusedHelper(): number {
  return checkedLive() + spare() + live();
}

// checkedLive is called by unusedHelper alone.
function checkedLive(): number {
  return live();
}

// FixtureCase is a type nothing names.
type FixtureCase = {
  name: string;
};
