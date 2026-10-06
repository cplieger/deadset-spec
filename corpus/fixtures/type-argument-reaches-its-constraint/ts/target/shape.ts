// Sizer reports a size.
interface Sizer {
  size(): number;
}

// Box is passed as a type argument and never converted to Sizer.
export class Box {
  size(): number {
    return 2;
  }
}

// total adds the sizes of its values, calling size through the constraint.
export function total<S extends Sizer>(...values: S[]): number {
  let sum = 0;
  for (const value of values) {
    sum += value.size();
  }
  return sum;
}
