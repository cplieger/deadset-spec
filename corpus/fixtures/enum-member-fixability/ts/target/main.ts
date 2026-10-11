import { Phase, Tier } from "./phase.js";

function isTerminal(p: Phase): boolean {
  return p === Phase.Failed || p === Phase.Running;
}

if (!isTerminal(Phase.Failed) || Tier.Top !== 7) {
  throw new Error("a failed phase is terminal");
}
