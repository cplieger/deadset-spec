import { count, emit, log, look } from "./wire.js";

// The entry file calls each function of the module once.

emit({ id: 1, inner: { label: "a" }, meta: { at: "now" } });
log({ line: "started" });
look();
count({ title: "a" });
