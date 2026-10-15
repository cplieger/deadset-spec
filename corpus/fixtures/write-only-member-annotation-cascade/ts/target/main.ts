type Stamp = {
  at: number;
};

class Clock {
  last: Stamp | undefined;
  tick(n: number): void {
    this.last = { at: n };
  }
}

new Clock().tick(1);
