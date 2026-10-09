export interface Store<S> {
  get<K extends keyof S>(key: K): S[K];
}

export declare function createStore<S>(): Store<S>;
