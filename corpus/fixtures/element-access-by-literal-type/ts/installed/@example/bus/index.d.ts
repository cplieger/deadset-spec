export interface Bus<M> {
  emit<K extends keyof M>(key: K, payload: M[K]): void;
  on<K extends keyof M>(key: K, fn: (payload: M[K]) => void): void;
  once<K extends Extract<keyof M, string>>(key: K): void;
  clear(event?: keyof M): void;
}

export declare function createBus<M>(): Bus<M>;
