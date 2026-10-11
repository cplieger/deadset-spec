import { inject } from "vitest";
import { warm } from "./support.ts";

warm();
console.log(inject("port"));
