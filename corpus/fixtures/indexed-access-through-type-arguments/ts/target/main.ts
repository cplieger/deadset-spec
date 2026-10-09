import { createStore } from "@example/store";

// Payloads is indexed through a generic alias whose body is a conditional type.
interface Payloads {
  open: number;
  close: string;
}

type Handler<K extends keyof Payloads> = Payloads[K] extends undefined
  ? () => void
  : (payload: Payloads[K]) => void;

function on<K extends keyof Payloads>(type: K, fn: Handler<K>): void {
  void type;
  void fn;
}

on("open", () => undefined);

// Operations is indexed through a generic alias written with a literal type argument.
interface Operations {
  rename: string;
  remove: number;
}

type Input<K extends keyof Operations> = Operations[K];

export const input: Input<"rename"> = "new";

// StoreMap is indexed through a dependency's generic declaration bound to it.
interface StoreMap {
  page: string;
  theme: string;
}

const store = createStore<StoreMap>();

export const page = store.get("page");
