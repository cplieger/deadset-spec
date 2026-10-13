// setup writes seen in a closure and reads it nowhere, and reads read in another.
function setup(): () => number {
  let seen: number;
  let read = 0;
  const later = (): void => {
    seen = 1;
    read = 2;
  };
  return () => {
    later();
    return read;
  };
}

export const result = setup()();
