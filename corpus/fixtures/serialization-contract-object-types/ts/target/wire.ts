// Payload is the static type of a value JSON.stringify serializes.
interface Payload {
  id: number;
  inner: Inner;
  meta: Meta | undefined;
}

// Inner is reached from a data member of Payload.
interface Inner {
  label: string;
}

// Meta is an object type a type alias declares, reached through a union.
type Meta = {
  at: string;
};

// Unsent is the type of no serialized value.
interface Unsent {
  title: string;
}

// Stamp carries a toJSON method, which the serializer calls.
class Stamp {
  at = 0;

  toJSON(): number {
    return this.at;
  }

  describe(): string {
    return String(this.at);
  }
}

// Logged is the static type of a value passed to a rest parameter typed any[]
// of a function the analyzed program does not declare.
interface Logged {
  line: string;
}

// Lookup holds a member read by name through two element accesses.
class Lookup {
  refresh(): number {
    return 1;
  }

  flush(): number {
    return 2;
  }
}

export function emit(payload: Payload): string {
  return JSON.stringify(payload) + JSON.stringify(new Stamp());
}

export function log(logged: Logged): void {
  console.log(logged);
}

export function look(): number {
  const table: Record<string, unknown> = {};
  const loose: any = new Lookup();
  return (table["refresh"] === undefined ? 0 : 1) + loose["flush"]();
}

export function count(value: Unsent): number {
  return value === undefined ? 0 : 1;
}
