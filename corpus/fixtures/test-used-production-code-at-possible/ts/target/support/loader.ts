function inner(): number {
  return 2;
}

export function load(): number {
  return inner();
}

export const spare = 3;
