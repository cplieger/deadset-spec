export function now(): number {
  return Date.now();
}

export const fixedNow = 1664379699000;

const deadOnly = 2;

export function stale(): number {
  return fixedNow + deadOnly;
}
