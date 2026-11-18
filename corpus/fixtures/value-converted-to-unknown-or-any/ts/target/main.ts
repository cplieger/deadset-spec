interface Note {
  id: string;
}

interface Track {
  title: string;
}

interface Label {
  text: string;
}

interface Signal {
  kind: string;
}

interface Reply {
  name: string;
}

interface Kept {
  count: number;
}

interface Endpoint {
  decode: (value: unknown) => unknown;
}

interface Checked {
  decode: (value: unknown) => Kept;
}

function decodeNote(value: unknown): Note {
  return { id: String(value) };
}

async function loadTrack(value: unknown): Promise<Track> {
  return { title: String(value) };
}

function decodeKept(value: unknown): Kept {
  return { count: Number(value) };
}

const endpoints: Record<string, Endpoint> = { note: { decode: decodeNote } };
const loaders: ((value: unknown) => Promise<any>)[] = [loadTrack];
const checked: Record<string, Checked> = { kept: { decode: decodeKept } };

let held: unknown;
const label: Label = { text: "l" };
held = label;

function emit(send: (value: unknown) => void): void {
  const signal: Signal = { kind: "s" };
  send(signal);
}

function reply(): unknown {
  const answer: Reply = { name: "r" };
  return answer;
}

emit(() => undefined);

export const results: unknown[] = [endpoints["note"]?.decode(1), loaders[0]?.(2), held, reply()];
export const decoded: boolean = checked["kept"]?.decode(3) !== undefined;
