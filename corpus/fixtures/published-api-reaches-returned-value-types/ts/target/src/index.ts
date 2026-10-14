export interface Meta {
  runtime: string;
}

type Static = Meta & {
  version?: string;
};

type Full = Meta & {
  date?: Date;
};

function created(): Static {
  const made: Static = { runtime: "node" };
  made.version = "1";
  return made;
}

const meta = created();

export function getMeta(): Meta {
  return Object.assign({}, meta, { date: new Date() }) as Full;
}
