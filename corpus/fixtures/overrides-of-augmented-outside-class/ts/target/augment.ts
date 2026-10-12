// The target adds a member to the dependency's class, which stays declared there.
export {};
declare module "@example/elements" {
  interface Element {
    describe(): string;
  }
}
