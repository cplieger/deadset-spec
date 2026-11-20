type Alias = string & {
  readonly alias: "Alias";
};

interface Label {
  readonly text?: string;
}

export function aliasOf(value: string): Alias | undefined {
  return value === "" ? undefined : (value as Alias);
}

export function labelOf(): Label {
  return {};
}
