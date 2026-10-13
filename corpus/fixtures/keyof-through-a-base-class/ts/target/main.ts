import { Emitter } from "./emitter.js";

interface Events {
  readonly open: [];
  readonly close: [];
}

// Stopper emits close through this.
class Stopper extends Emitter<Events> {
  stop(): boolean {
    return this.emit("close");
  }
}

export const stopped = new Stopper().stop();
