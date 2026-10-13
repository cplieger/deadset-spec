interface Base {
  readonly kind: string;
}

interface Action extends Base {
  readonly kind: string;
  readonly spare: string;
}

// describe reads kind through its constraint.
function describe<T extends Base>(value: T): string {
  return value.kind;
}

const action: Action = { kind: "a", spare: "s" };
export const described = describe(action);
