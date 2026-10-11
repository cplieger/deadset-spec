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

// measure is called by name, so its pattern is judged by the names it binds.
function measure(
  {
    left,
    right,
  }: { readonly left: number; readonly right: number },
): number {
  return left;
}

// spread reads its rest element, so the name beside it is not reported.
function spread(
  {
    left: kept,
    ...rest
  }: { readonly left: number; readonly right: number },
): number {
  return rest.right;
}

export const measured = measure({ left: 1, right: 2 }) + spread({ left: 1, right: 2 });
