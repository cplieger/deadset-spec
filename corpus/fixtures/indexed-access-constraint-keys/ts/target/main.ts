type Kind = "a" | "b";

interface Payload {
  readonly a: string;
  readonly b: number;
  readonly c: boolean;
}

interface Handlers {
  readonly click: string;
  readonly key: string;
  readonly [slot: number]: string;
}

// pick indexes Payload by a type parameter Kind constrains.
function pick<K extends Kind>(): Payload[K] {
  return undefined as never;
}

const written: Handlers["click" | number] = "";
export const read = [pick<"a">(), written];
