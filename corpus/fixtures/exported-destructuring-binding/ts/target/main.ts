import { useField } from "./hooks.js";

if (useField() !== 1) {
  throw new Error("useField is not 1");
}
