import { keyed, send } from "./lib.js";

interface Payload {
  readonly title: string;
}

interface Keyed {
  readonly id: string;
  readonly note: string;
}

const payload: Payload = { title: "t" };
const record: Keyed = { id: "i", note: "n" };
await send("u", payload);
keyed(record);
