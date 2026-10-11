interface Saved {
  rules: string[];
  fallback: string;
}

interface Wrapped {
  inner: number;
}

interface Envelope {
  kind: string;
  payload: unknown;
}

interface Listed {
  value: number;
}

interface Boxed {
  size: number;
}

interface Kept {
  note: string;
}

interface Bag {
  [key: string]: unknown;
}

const saved: Saved = { rules: ["en"], fallback: "en" };
const sections: Record<string, unknown> = {};
sections["languages"] = saved;

const wrapped: Wrapped = { inner: 1 };
const envelope: Envelope = { kind: "w", payload: wrapped };

const listed: Listed = { value: 2 };
const items: unknown[] = [listed];

const boxed: Boxed = { size: 3 };
let box: unknown;
box = boxed;

const kept: Kept = { note: "k" };
const scratch: Bag = {};
scratch["kept"] = kept;

export const body: string = JSON.stringify(sections) + JSON.stringify(envelope) + JSON.stringify(items) + JSON.stringify(box);
export const held: number = Object.keys(scratch).length;
