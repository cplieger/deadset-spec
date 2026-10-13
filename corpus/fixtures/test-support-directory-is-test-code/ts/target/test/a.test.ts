import { onlyTested } from "../src/only-tested.js";
import { make } from "./helpers/make.js";

export function testMake(): void {
  console.log(make(), onlyTested());
}
