import { load } from "./support/index.js";

export function testLoad(): boolean {
  return load() > 0;
}
