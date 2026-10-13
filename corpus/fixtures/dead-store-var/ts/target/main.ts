// hoisted writes value twice before it reads it.
function hoisted(): number {
  var value = 1;
  value = 2;
  return value;
}

// nested overwrites later in a block and reads it after the block.
function nested(flag: boolean): number {
  if (flag) {
    var later: number | undefined = 1;
    later = 2;
  }
  return later ?? 0;
}

export const values = hoisted() + nested(true);
