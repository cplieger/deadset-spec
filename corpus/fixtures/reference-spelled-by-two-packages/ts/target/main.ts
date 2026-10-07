import { sent as first } from "./examples/a/index.js";
import { sent as second } from "./examples/b/index.js";
import { Unsent } from "./unsent.js";

const unsent = new Unsent();
console.log(first, second, typeof unsent);
