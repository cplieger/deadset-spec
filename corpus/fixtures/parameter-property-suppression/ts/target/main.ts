import { Dial, Gauge, Meter } from "./meters.js";

// The entry file constructs each class once.
const gauge = new Gauge("kg", 2);
const meter = new Meter("m", 3);
console.log(typeof gauge, typeof meter, new Dial("deg").describe());
