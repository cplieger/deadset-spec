// apply calls the function it is handed.
function apply(f: (name: string, n: number) => boolean): boolean {
  return f("b", 2);
}

// check is a function expression the program only calls.
const check = (name: string, n: number): boolean => n > 0;

export const results = [
  check("a", 1),
  apply(
    (name, n) => n > 1,
  ),
];
