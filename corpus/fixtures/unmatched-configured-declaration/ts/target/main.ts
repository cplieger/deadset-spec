import { Envelope, encode } from "./codec.js";

// The entry file passes an instance of the class to the configured serializer.
if (encode(new Envelope()) === "") {
  throw new Error("the envelope encoded to nothing");
}
