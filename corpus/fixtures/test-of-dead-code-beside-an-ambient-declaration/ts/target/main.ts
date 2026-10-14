// The host supplies platform, which the package publishes and calls.
export declare function platform(): number;

import { live } from "./catalog.js";

console.log(platform(), live());
