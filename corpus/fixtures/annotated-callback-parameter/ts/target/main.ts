import { apply, infer } from "./calls.js";

export const total =
  apply((
    a: number,
    unread: number,
  ) => a) +
  infer((
    label: string,
  ) => 0).length;
