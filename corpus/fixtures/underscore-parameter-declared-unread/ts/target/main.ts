import { apply } from "./calls.js";

function plain(value: number, unused: number): number {
  return value;
}

function trailing(value: number, _unused: number): number {
  return value;
}

function picked([_skip, kept]: [number, number]): number {
  return kept;
}

export const total =
  plain(1, 2) +
  trailing(1, 2) +
  picked([1, 2]) +
  apply((a: number, _b: number) => a);
