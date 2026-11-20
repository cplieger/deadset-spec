interface Branded {
  readonly named: "Named";
}

type Alias = string & { readonly alias: "Alias" };

type Count = number & Branded;

type Flag = boolean & { readonly flag: "Flag" };

interface Tag {
  readonly label?: string;
}

interface Fact {
  readonly alias?: Alias | undefined;
  readonly counts?: readonly Count[];
  readonly flag?: Flag;
  readonly tag?: Tag;
}

export interface Model {
  readonly facts: ReadonlyMap<string, Fact>;
}

export const model: Model = { facts: new Map() };
