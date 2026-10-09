import * as decoders from "./decoders.js";
import * as named from "./named.js";

for (const [name, decode] of Object.entries(decoders)) {
  console.log(name, decode("x"));
}
console.log(named.used());
