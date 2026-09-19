// The module file the entry and the test file import. It is named by no manifest
// entry point, so its exports are not roots and the analysis judges them by their
// references.

// onlyTested is referenced from the test file alone.
export function onlyTested(): number {
  return 1;
}

// production is referenced from the entry file as well as from the test file.
export function production(): number {
  return 2;
}
