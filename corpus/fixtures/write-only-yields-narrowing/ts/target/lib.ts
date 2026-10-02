// Written by bump and read by nothing.
export let hits = 0;

// Records a hit, and the consumer calls it.
export function bump(): void {
  hits = 1;
}
