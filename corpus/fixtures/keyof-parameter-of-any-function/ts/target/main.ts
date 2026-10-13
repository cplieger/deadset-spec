interface Events {
  readonly closed: string;
  readonly opened: string;
}

interface Options {
  readonly a: string;
  readonly b: string;
}

// clear reads the key it is passed.
function clear(event: keyof Events): string {
  return event;
}

declare function createStore<M>(): { get<K extends keyof M>(key?: K): M[K] };

export const read = [clear("closed"), createStore<Options>().get("a")];
