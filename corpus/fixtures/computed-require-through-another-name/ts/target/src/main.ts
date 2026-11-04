import { createRequire } from "node:module";

function deadCaller(): number {
  return calledOnlyByDead();
}

function calledOnlyByDead(): number {
  return 1;
}

function label(value: number): string {
  return String(value);
}

const load = createRequire(import.meta.url);

export function start(name: string): unknown {
  return [load(name), label(2)];
}

void start("a");
