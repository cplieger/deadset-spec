import { check } from "absent-runner";

export function testSum(): void {
  const ignored = 1 + 1;
  check(1 + 1 === 2);
}
