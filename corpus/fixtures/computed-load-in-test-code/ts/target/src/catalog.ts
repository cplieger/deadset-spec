// Only the entry file imports this module; the test file's glob loads it as well.

export function production(): number {
  return 1;
}

export function onlyTested(): number {
  return 2;
}
