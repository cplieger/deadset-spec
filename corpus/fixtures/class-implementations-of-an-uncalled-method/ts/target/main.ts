import { Cache, clear, Disk, Memory, read } from "./store.js";

const disk = new Disk("a");
disk.reset();
const cache = new Cache();
clear(cache);
if (read(new Memory(), "a") + read(disk, "b") + read(cache, "c") < 0) {
  throw new Error("a negative read");
}
