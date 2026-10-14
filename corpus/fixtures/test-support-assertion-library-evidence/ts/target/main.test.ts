import { expectOk } from "./support/assert.js";
import { plain } from "./support/plain.js";
import { stubbed } from "./support/stub.js";
import { timed } from "./support/timers.js";

expectOk(plain() > 0);
console.log(stubbed(), timed());
