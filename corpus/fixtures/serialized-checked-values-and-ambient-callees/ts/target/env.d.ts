interface Client {
  postMessage(message: unknown): void;
}

declare function relay(message: unknown): void;

interface Sink {
  put<T>(message: T): void;
}
