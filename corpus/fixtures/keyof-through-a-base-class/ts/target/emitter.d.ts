// Emitter types the events of its subclasses by its type argument.
export declare class Emitter<T> {
  emit<K extends keyof T>(event: K): boolean;
}
