// encode reads the data members of the object it is given by name, which its
// declaration does not say, so the configuration names it as a serializer.
export function encode(value: object): string {
  return Object.keys(value).join(",");
}

// Envelope is the class of the value passed to encode.
export class Envelope {
  body = "";
}
