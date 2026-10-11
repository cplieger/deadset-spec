export interface Store<S> {
  get<K extends keyof S>(key: K): S[K];
  has<K extends keyof S>(key: K): boolean;
}

export declare function createStore<S>(): Store<S>;
