interface Scope {
  onconnect: ((event: MessageEvent) => void) | null;
}
(globalThis as unknown as Scope).onconnect = (event) => {
  console.log(event.data);
};

interface Shown {
  onclick: (() => void) | null;
}
interface ShownCtor {
  new (title: string): Shown;
}
interface Env {
  readonly ctor: ShownCtor;
}
export function show(env: Env): void {
  const posted = new env.ctor("t");
  posted.onclick = () => undefined;
}

interface Decoded {
  name: string;
  spare: string;
}
export function decode(text: string): string {
  const decoded = JSON.parse(text) as Decoded;
  decoded.spare = "";
  return decoded.name;
}
