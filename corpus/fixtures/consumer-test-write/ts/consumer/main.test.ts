import { defaults } from "../target/options.js";

// The consumer's test sets a label on the target's options.
export function testLabel(): void {
  const options = defaults();
  options.label = "test";
}
