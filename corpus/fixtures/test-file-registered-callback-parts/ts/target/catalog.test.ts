import { live } from "./catalog.js";

// register stands for a test runner's registration call: the runner calls the
// function each call hands it.
function register(name: string, body: () => void): void {
  if (name.length === 0) {
    throw new Error("a test needs a name");
  }
  body();
}

register("live returns one", () => {
  const unused = live();
  const value = live();
  if (value !== 1) {
    throw new Error("live is not 1");
  }
});
