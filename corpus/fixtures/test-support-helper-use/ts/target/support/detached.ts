// detached is called by a test and by a test-file helper.
export function detached(): number {
  return 7;
}

// Sink is the parameter type of drain, which only tests call.
export interface Sink {
  put(v: number): void;
}

// drain puts one value into the sink a test hands it.
export function drain(s: Sink): void {
  s.put(1);
}
