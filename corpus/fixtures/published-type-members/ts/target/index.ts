import type { Entry, Loaded, Options } from "./options.js";

// Internal is a type only this module's body names.
interface Internal {
  count: number;
  spare?: number;
}

// configure is the published entry, and its parameter's type is declared in another module.
export function configure(options: Options): number {
  const local: Internal = { count: options.name.length };
  return local.count;
}

// load is published, and its result type names a type argument declared in another module.
export async function load(): Promise<Loaded[]> {
  return [{ id: 1 }];
}

// Registry is published, and the type of its property is an array of a type declared in another module.
export class Registry {
  entries: Entry[] = [];
}
