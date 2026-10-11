function touch(value: number): void {
  console.log(value);
}

// deadset:ignore DS1801 -- The parameter is kept on purpose.
function parameterAbove(value: number, label: string): number {
  return value * 2;
}

function parameterTrailing(value: number, label: string): number { // deadset:ignore DS1801 -- The parameter is kept on purpose.
  return value * 2;
}

function parameterEntry(value: number, label: string): number {
  return value * 2;
}

// deadset:ignore DS1801 -- The parameter is kept on purpose.
function parameterLive(value: number, label: string): number {
  return value + label.length;
}

// deadset:ignore DS1803 -- The result is kept on purpose.
function resultAbove(): number {
  return 1;
}

function resultTrailing(): number { // deadset:ignore DS1803 -- The result is kept on purpose.
  return 1;
}

function resultEntry(): number {
  return 1;
}

// deadset:ignore DS1803 -- The result is kept on purpose.
function resultLive(): number {
  return 2;
}

// deadset:ignore DS1805 -- The statement is kept on purpose.
function statementAbove(): void {
  touch(0);
  return;
  touch(1);
}

function statementTrailing(): void { // deadset:ignore DS1805 -- The statement is kept on purpose.
  touch(0);
  return;
  touch(1);
}

function statementEntry(): void {
  touch(0);
  return;
  touch(1);
}

// deadset:ignore DS1805 -- The statement is kept on purpose.
function statementLive(): void {
  touch(0);
}

// deadset:ignore DS1807 -- The store is kept on purpose.
function storeAbove(): number {
  let count = 1;
  count = 2;
  return count;
}

function storeTrailing(): number { // deadset:ignore DS1807 -- The store is kept on purpose.
  let count = 1;
  count = 2;
  return count;
}

function storeEntry(): number {
  let count = 1;
  count = 2;
  return count;
}

// deadset:ignore DS1807 -- The store is kept on purpose.
function storeLive(): number {
  let count = 1;
  count += 2;
  return count;
}

// deadset:ignore DS1809 -- The case is kept on purpose.
function caseAbove(v: string): string {
  switch (v) {
    case "node":
      return "first";
    case "node":
      return "second";
  }
  return "other";
}

function caseTrailing(v: string): string { // deadset:ignore DS1809 -- The case is kept on purpose.
  switch (v) {
    case "node":
      return "first";
    case "node":
      return "second";
  }
  return "other";
}

function caseEntry(v: string): string {
  switch (v) {
    case "node":
      return "first";
    case "node":
      return "second";
  }
  return "other";
}

// deadset:ignore DS1809 -- The case is kept on purpose.
function caseLive(v: string): string {
  switch (v) {
    case "node":
      return "first";
    case "edge":
      return "second";
  }
  return "other";
}

export const out: unknown[] = [];
out.push(parameterAbove(1, "a"));
out.push(parameterTrailing(1, "a"));
out.push(parameterEntry(1, "a"));
out.push(parameterLive(1, "a"));
resultAbove();
resultTrailing();
resultEntry();
out.push(resultLive());
statementAbove();
statementTrailing();
statementEntry();
statementLive();
out.push(storeAbove());
out.push(storeTrailing());
out.push(storeEntry());
out.push(storeLive());
out.push(caseAbove("node"));
out.push(caseTrailing("node"));
out.push(caseEntry("node"));
out.push(caseLive("node"));
