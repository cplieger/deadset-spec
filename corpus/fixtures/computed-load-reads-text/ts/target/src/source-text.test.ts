// The test reads every module of its directory as text, which uses none of their declarations.
const sources = import.meta.glob(["./*.ts"], { query: "?raw", import: "default", eager: true });
if (Object.keys(sources).length === 0) {
  throw new Error("no sources");
}
