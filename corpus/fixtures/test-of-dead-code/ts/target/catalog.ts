// The module file the entry and the test file import. It is named by no manifest
// entry point, so its exports are not roots and the analysis judges them by their
// references.

// deadOne has no reference from a production file.
export function deadOne(): number {
  return 1;
}

// deadTwo has no reference from a production file.
export function deadTwo(): number {
  return 2;
}

// live is referenced from the entry file.
export function live(): number {
  return 3;
}
