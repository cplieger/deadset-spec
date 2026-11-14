import { register } from "./registry.js";

// main hands one module's namespace on, binds another and reads one export through the
// binding, reads one export of a third by name and loads a fourth.
async function main(): Promise<void> {
  register(import("./routes.js"));
  const widgets = await import("./widgets.js");
  if (widgets.used() === 0) {
    throw new Error("no widgets");
  }
  console.log((await import("./gadgets.js")).shown());
  await import("./effects.js");
}

void main();
