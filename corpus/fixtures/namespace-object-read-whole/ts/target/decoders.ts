// decodeA is read through the namespace object alone.
export function decodeA(raw: string): string {
  return raw;
}

// decodeB is read through the namespace object alone.
export function decodeB(raw: string): string {
  return raw + raw;
}
