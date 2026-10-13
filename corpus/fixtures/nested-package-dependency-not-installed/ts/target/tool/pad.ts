import leftPad from "left-pad";

// pad hands its value to the dependency nothing installs.
export function pad(value: string): string {
  return leftPad(value, 4);
}

// unused is called by nothing.
function unused(): number {
  return 1;
}
