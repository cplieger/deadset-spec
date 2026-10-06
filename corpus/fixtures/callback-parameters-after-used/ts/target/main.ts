import { apply, type Box, each, withBox, withPair, withScaled } from "./calls.js";

// scale is called by name, so its signature is free and a parameter its body never
// reads is reported whatever its name.
function scale(
  factor: number,
  _unused: number,
): number {
  return factor * 2;
}

// Each callback below is used as a value, and a caller may pass a function more
// arguments than it declares, so only an unread parameter that no read parameter
// follows is reported.
const total =
  apply(
    (
      first,
      second,
      third,
    ) => second,
    1,
  ) +
  each(
    (
      value,
    ) => 0,
  ) +
  each(
    (
      _skipped,
      kept,
    ) => kept,
  ) +
  withPair(
    (
      {
        left,
        right,
      },
      weight,
    ) => left * weight,
  ) +
  withScaled(
    (
      factor,
      {
        left,
      },
    ) => left,
  ) +
  withPair(
    (
      {
        left: dropped,
        ...others
      },
    ) => others.right,
  ) +
  withPair(
    (
      {
        left: leftOnly,
        ...unreadRest
      },
    ) => leftOnly,
  ) +
  apply(
    (
      head,
      ...tail
    ) => head,
    1,
  ) +
  apply(
    (
      skippedHead,
      ...readTail
    ) => readTail.length,
    1,
  ) +
  withBox(function (
    this: Box,
  ): number {
    return 0;
  });

if (total + scale(2, 3) < 0) {
  throw new Error("the total is negative");
}
