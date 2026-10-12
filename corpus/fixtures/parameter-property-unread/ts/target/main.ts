import { Box, Shelf } from "./box.js";

// The entry file reads three parameter properties of a box, each by its own route.
const box = new Box("crate", "spare", 2, 3, 4, 5);
const { height } = box;
console.log(box.describe(), box.width, height, box["depth"]);

// The shelf class is used as a value, so its constructor's signature is fixed.
const make = Shelf;
console.log(new make(1));
