// Store finds a value by name.
interface Store {
  find(name: string): string;
}

// Plain is the store the entry converts to Store.
export class Plain implements Store {
  find(name: string): string {
    return name;
  }
}

// Encrypted is built by makeEncrypted alone.
class Encrypted implements Store {
  constructor(private readonly key: string) {}

  find(name: string): string {
    return this.key + name;
  }
}

// makeEncrypted converts an encrypted store to Store, and nothing calls it.
function makeEncrypted(): Store {
  return new Encrypted("k");
}

// use finds one name through the interface.
export function use(store: Store): string {
  return store.find("x");
}
