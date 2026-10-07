import { encode } from "./codec.js";

// First is sent through encode.
class First {
  body = "first";
}

export const sent = encode(new First());
