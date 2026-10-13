interface Sig {
  readonly a: string;
  readonly b: string;
  readonly c: string;
  readonly d: string;
}

// Box indexes Sig by the key its constructor is passed.
class Box<K extends keyof Sig> {
  constructor(readonly key: K) {}
  value?: Sig[K];
}

declare const make: new <K extends keyof Sig>(key: K) => { v: Sig[K] };

const holder = { pick: <K extends keyof Sig>(): Sig[K] => undefined as never };

const box = new Box("a");
export const read = [box.key, box.value, new make("b").v, holder.pick<"c">()];
