import { Disk, total } from "./store.js";

if (total(new Disk()) !== 2) {
  throw new Error("total() is not 2");
}
