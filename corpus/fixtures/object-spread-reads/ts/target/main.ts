import { Counter, Gauge, type Shape, type Sized } from "./counter.js";

const frozen: Readonly<Counter> = new Counter();
const mapped = { ...frozen };
const direct = { ...new Gauge() };
const square: Shape = { side: 2, area: () => 4 };
const copied = { ...square };
const box: Sized = { size: 3 };
const measured = { ...box };

if (mapped.count + direct.level + frozen.doubled === 0 || copied === square || measured === box) {
  throw new Error("a counter counts");
}
