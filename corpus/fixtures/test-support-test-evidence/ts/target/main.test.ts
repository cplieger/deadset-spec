import { bare } from "./support/bare.js";
import { mocked } from "./support/server.js";
import { rendered } from "./support/render.js";
import { shared } from "./support/shared.js";

console.log(bare(), mocked(), rendered(), shared());
