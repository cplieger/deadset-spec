export class Counter {
  count = 1;
  label = "counter";
  get doubled(): number {
    return this.count * 2;
  }
  get stale(): number {
    return 0;
  }
}

export class Gauge {
  level = 2;
  get spent(): number {
    return 0;
  }
}

export interface Shape {
  side: number;
  area(): number;
}

export interface Sized {
  get size(): number;
}
