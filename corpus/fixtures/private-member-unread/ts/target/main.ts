import { Registry } from "./registry.js";

// The entry file constructs the class and reaches one private member by name,
// which is the only reference that member has.
const registry = new Registry("target");
if (registry["label"] === "") {
  throw new Error("the registry carries an empty label");
}
