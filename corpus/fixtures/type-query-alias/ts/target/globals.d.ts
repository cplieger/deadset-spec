export {};

declare global {
  const useCounter: typeof import("./counter.js")["useCounter"];
  const resetCounter: typeof import("./counter.js")["resetCounter"];
}
