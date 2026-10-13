import { value as one } from "dead-only";
import { value as two } from "reached";
import { value as four } from "live";
import { value as five } from "file-level";

void five;

export function first(): number {
  return one + four + inner();
}

export function second(): number {
  return one;
}

function inner(): number {
  return two;
}
