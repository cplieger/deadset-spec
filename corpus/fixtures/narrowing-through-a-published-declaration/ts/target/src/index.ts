import { describe } from "./describe.js";

// Hint is the type of the exported property Me.hint.
export class Hint {}
// Card is the element of the exported property Me.cards.
export class Card {}
// Note is the map value of the exported property Me.notes.
export class Note {}
// Badge is the class Me extends.
export class Badge {}
// Inner is the type of a private property of Me only.
export class Inner {}

// Me is what fetch returns.
export class Me extends Badge {
  hint = new Hint();
  cards: Card[] = [new Card()];
  notes = new Map<string, Note>([["a", new Note()]]);
  private inner = new Inner();
  holds(): boolean {
    return this.inner instanceof Inner;
  }
}

// Request is the parameter of send.
export interface Request {
  path: string;
}

// Config is the type of the exported constant defaults.
export interface Config {
  port: number;
}
export const defaults: Config = { port: 1 };

// Job is the element of the exported type Queue, which jobs returns.
export class Job {}
export type Queue = readonly Job[];

// Local is named only inside the package.
export class Local {}

// Tag is the type of a property of Summary, which summary returns unexported.
export class Tag {}
class Summary {
  tag = new Tag();
}

// Made is what make returns, with no annotation naming it.
export class Made {}

export function fetch(): Me {
  return new Me();
}
export function send(request: Request): string {
  return request.path + String(defaults.port) + describe(new Me(), new Local());
}
export function jobs(): Queue {
  return [new Job()];
}
export function summary(): Summary {
  return new Summary();
}
export function make() {
  return new Made();
}

// Base is the class Impl extends, which build returns unexported.
export class Base {}
class Impl extends Base {}

// Elem is the element of Pair, an unexported alias pair returns.
export class Elem {}
type Pair = readonly Elem[];

// Shape is the interface Wide extends, which wide returns unexported.
export interface Shape {
  s: number;
}
interface Wide extends Shape {
  w: number;
}

// Indexed is the value of Bucket's index signature, which bucket returns.
export class Indexed {}
export interface Bucket {
  [key: string]: Indexed;
}

// Held is what the namespace's function Ns.held returns.
export class Held {}

export function build(): Impl {
  return new Impl();
}
export function pair(): Pair {
  return [new Elem()];
}
export function wide(): Wide {
  return { s: 1, w: 2 };
}
export function bucket(): Bucket {
  return { a: new Indexed() };
}
export namespace Ns {
  export function held(): Held {
    return new Held();
  }
}

// Pred is the type Pred's guard narrows to.
export class Pred {}
// Asserted is the type an assertion function narrows to.
export class Asserted {}
// Bound is the constraint of bounded's type parameter.
export class Bound {}
// Fallback is the default of fallback's type parameter.
export class Fallback {}
// Yes and No are the branches of Choice, an unexported conditional choose returns.
export class Yes {}
export class No {}
type Choice<T> = T extends string ? Yes : No;
// Template is the template of Each, an unexported mapped type each returns.
export class Template {}
type Each<T> = { [K in keyof T]: Template };
// Id is what id returns, an alias the checker reads as string.
export type Id = string;
// Contract is the interface Signed implements, which signed returns.
export interface Contract {
  c: number;
}
export class Signed implements Contract {
  c = 1;
}
// Limit is the constraint of Crate's type parameter, which crate returns.
export class Limit {}
export class Crate<T extends Limit> {
  constructor(readonly v: T | undefined) {}
}

export function isPred(value: unknown): value is Pred {
  return value instanceof Pred;
}
export function assertAsserted(value: unknown): asserts value is Asserted {
  if (!(value instanceof Asserted)) {
    throw new Error("not asserted");
  }
}
export function bounded<T extends Bound>(value: T): T {
  return value;
}
export function fallback<T = Fallback>(value?: T): T | undefined {
  return value;
}
export function choose<T>(value: T): Choice<T> {
  return (typeof value === "string" ? new Yes() : new No()) as Choice<T>;
}
export function each(): Each<{ a: number }> {
  return { a: new Template() };
}
export function id(): Id {
  return "id";
}
function contractOf(value: Contract): number {
  return value.c;
}
export function signed(): Signed {
  const made = new Signed();
  return contractOf(made) === 1 ? made : new Signed();
}
export function crate(): Crate<never> {
  return new Crate<never>(undefined);
}
