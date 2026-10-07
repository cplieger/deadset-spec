// encode writes a value as the wire format.
export function encode(value: object): string {
  return JSON.stringify(value);
}
