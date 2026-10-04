export interface Entry {
  readonly key: string;
  readonly note: string;
  readonly tag: string;
  readonly spare?: string;
}

export interface Row {
  readonly id: number;
  readonly source: string;
}

// The annotated literal is checked for excess properties.
export function entryOf(key: string): Entry {
  const entry: Entry = { key, note: "a note", tag: "t" };
  return entry;
}

// The callback's literal takes its type from the type argument and is not checked for excess properties.
export function rowsOf(ids: readonly number[]): Row[] {
  return ids.map<Row>((id) => ({ id, source: "list" }));
}
