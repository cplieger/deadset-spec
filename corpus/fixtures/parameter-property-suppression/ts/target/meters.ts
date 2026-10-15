// A directive on a parameter property binds its member and the constructor's parameters.
export class Gauge {
  constructor(
    // deadset:ignore DS1301,DS1401 -- Kept for the next release of the gauge.
    private readonly unit: string,
    scale: number,
  ) {
    console.log("gauge");
  }
}

// Ignore entries bind the same two codes, the member's by its reference and the parameter's by the class's.
export class Meter {
  constructor(
    private readonly unit: string,
    scale: number,
  ) {
    console.log("meter");
  }
}

// A parameter property is never an unused parameter, so this directive matches nothing.
export class Dial {
  constructor(
    // deadset:ignore DS1401 -- Kept for the next release of the dial.
    private readonly unit: string,
  ) {
    console.log("dial");
  }

  describe(): string {
    return this.unit;
  }
}
