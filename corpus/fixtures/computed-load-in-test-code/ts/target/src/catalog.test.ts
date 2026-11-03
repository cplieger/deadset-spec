// The glob is written in a test file, so it loads each module for the tests alone.
const modules = import.meta.glob("./*.ts", { eager: true });
const catalog = modules["./catalog.ts"] as { onlyTested(): number };
if (catalog.onlyTested() !== 2) {
  throw new Error("onlyTested() is not 2");
}
