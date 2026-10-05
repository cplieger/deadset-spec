import { newMemory } from "./fake/fake.js";
import { total } from "./store.js";

if (total(newMemory()) !== 8) {
  throw new Error("total() is not 8");
}
