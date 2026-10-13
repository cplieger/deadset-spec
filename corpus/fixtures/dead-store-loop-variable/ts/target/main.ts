// sum reads value and name, and neither key, k nor _.
function sum(map: Map<string, number>, record: Record<string, number>): number {
  let total = 0;
  for (const [
    key,
    value,
  ] of map) {
    total += value;
  }
  for (const k in record) {
    total += 1;
  }
  for (const name in record) {
    total += record[name] ?? 0;
  }
  for (const _ of map) {
    total += 1;
  }
  return total;
}

export const summed = sum(new Map([["a", 1]]), { b: 2 });
