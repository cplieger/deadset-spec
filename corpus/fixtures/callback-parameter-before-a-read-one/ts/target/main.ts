import { apply } from "./calls.js";

export const total =
  apply((
    first,
    second,
  ) => second) +
  apply((
    _first,
    second,
  ) => second);
