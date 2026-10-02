import { make, shape, type Maker } from "./maker.js";

// The entry file calls both functions of the module and the method of the interface.
const maker: Maker = make();
shape<boolean>();
if (maker.build<number>() === "") {
  throw new Error("the maker built nothing");
}
