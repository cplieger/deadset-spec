import { encode } from "./codec.js";

// Second is sent through encode.
class Second {
  body = "second";
}

export const sent = encode(new Second());
