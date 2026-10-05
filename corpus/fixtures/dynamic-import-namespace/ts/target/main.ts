import { register } from "./registry.js";

// main hands one module's namespace on, reads one export of another and loads a third.
async function main(): Promise<void> {
  register(import("./routes.js"));
  const widgets = await import("./widgets.js");
  if (widgets.used() === 0) {
    throw new Error("no widgets");
  }
  await import("./effects.js");
}

void main();
