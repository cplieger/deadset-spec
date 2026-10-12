// Source is the type of the value passed to take.
interface Source {
  id: number;
  extra: number;
}
// The position is a mapped type over one literal key, whose property has no declaration.
function take(target: Record<"id", number>): number {
  return target.id;
}
const source: Source = { id: 1, extra: 2 };
console.log(take(source));
