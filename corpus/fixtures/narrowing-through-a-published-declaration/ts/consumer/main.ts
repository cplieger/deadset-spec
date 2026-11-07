import {
  assertAsserted,
  bounded,
  bucket,
  build,
  choose,
  crate,
  defaults,
  each,
  fallback,
  fetch,
  id,
  isPred,
  jobs,
  make,
  Ns,
  pair,
  send,
  signed,
  summary,
  wide,
} from "../target/src/index.js";

const me = fetch();
console.log(me.hint, me.cards, me.notes, me.holds());
console.log(send({ path: "/" }), defaults, jobs(), summary().tag, make());
const shape = wide();
console.log(build(), pair(), shape.s + shape.w, bucket(), Ns.held());
const value: unknown = me;
assertAsserted(value);
console.log(isPred(value), bounded(me), fallback(), choose("x"), each().a, id(), signed().c, crate().v);
