import { load } from "./support/loader.js";

export function testLoad(): boolean {
  return load() > 0;
}
