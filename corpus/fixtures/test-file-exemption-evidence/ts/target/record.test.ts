import { Sample } from "./record.js";

export function testSampleSerializes(): void {
  if (JSON.stringify(new Sample()) === "") {
    throw new Error("the sample serialized to nothing");
  }
}
