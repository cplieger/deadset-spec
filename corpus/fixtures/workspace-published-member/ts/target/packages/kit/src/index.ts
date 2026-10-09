import { assist } from "./internal.js";

// used is published, and the application calls it.
export function used(): number {
  return assist();
}

// spare is published, and nothing calls it.
export function spare(): number {
  return 2;
}
