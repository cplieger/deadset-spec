// scaled names a label its body never reads.
function scaled(value: number, label: string): number {
  return value * 2;
}

// doubled names one parameter its body reads.
function doubled(value: number): number {
  return value * 2;
}

// tally returns a count every call drops.
function tally(): number {
  return 1;
}

// total returns a count one call reads.
function total(): number {
  return 2;
}

// halted holds a statement after its return.
function halted(): void {
  doubled(0);
  return;
  doubled(1);
}

// overwritten stores twice before anything reads the local.
function overwritten(): number {
  let count = 1;
  count = 2;
  return count;
}

// classify names one value in two cases of one switch.
function classify(v: string): string {
  switch (v) {
    case "node":
      return "first";
    case "node":
      return "second";
  }
  return "other";
}

tally();
halted();
export const results = [scaled(1, "a"), doubled(2), total(), overwritten(), classify("node")];
