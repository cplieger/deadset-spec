import { keep } from "@example/keep";

interface Credential {
  readonly kind: string;
}
interface CredentialInit {
  id: string;
  password: string;
  meta: {
    origin: string;
  };
}
interface CredentialCtor {
  new (init: CredentialInit): Credential;
}
const Ctor = (globalThis as unknown as { Cred: CredentialCtor }).Cred;
export const made = new Ctor({ id: "a", password: "b", meta: { origin: "o" } }).kind;

interface StoreInit {
  key: string;
  secret: string;
}
const vault = (globalThis as unknown as { vault: { put(init: StoreInit): void } }).vault;
vault.put({ key: "k", secret: "s" });

host.share({ title: "t", url: "u" });

notify({ subject: "s", body: "b" });

interface Kept {
  shown: string;
  hidden: string;
}
export const kept = keep<Kept>({ shown: "x", hidden: "y" }).shown;

interface Item {
  label: string;
  note: string;
}
const items: Item[] = [];
items.push({ label: "l", note: "n" });
export const firstLabel = items[0]?.label;

interface Entry {
  label: string;
  note: string;
}
const cache = new Map<string, Entry>();
cache.set("k", { label: "l", note: "n" });
export const cachedLabel = cache.get("k")?.label;

interface Ping {
  ping: string;
}
interface Pong {
  pong: string;
}
declare function post(m: Ping | Pong): void;
post({ ping: "p" });
interface Named {
  name: string;
}
interface Sized {
  size: number;
}
declare function tag(t: Named & Sized): void;
tag({ name: "n", size: 1 });

interface Labelled {
  label: string;
  extra: string;
}
function format(a: Labelled): string;
function format(a: string): string;
function format(a: Labelled | string): string {
  return typeof a === "string" ? a : a.label;
}
export const formatted = format({ label: "l", extra: "e" });

interface Evt {
  kind: string;
  detail: string;
}
abstract class Handler {
  abstract handle(e: Evt): string;
}
class KindHandler extends Handler {
  handle(e: Evt): string {
    return e.kind;
  }
}
const handler: Handler = new KindHandler();
export const handled = handler.handle({ kind: "k", detail: "d" });

interface LocalInit {
  name: string;
  token: string;
}
type Sink = (init: LocalInit) => string;
const sink: Sink = (init) => init.name;
export const sunk = sink({ name: "n", token: "t" });

const local: Relay = {
  send(m) {
    return m.topic;
  },
};
export const relayed = local.send({ topic: "t", body: "b" });

interface Message {
  topic: string;
  body: string;
}
interface Channel {
  send(m: Message): string;
}
class LocalChannel implements Channel {
  send(m: Message): string {
    return m.topic;
  }
}
const remote = (globalThis as unknown as { remote: Channel }).remote;
const channel: Channel = Date.now() > 0 ? new LocalChannel() : remote;
export const sent = channel.send({ topic: "t", body: "b" });

interface Login {
  id: string;
  password: string;
}
function login(init: Login): string {
  return init.id;
}
export const loggedIn = login({ id: "i", password: "p" });
