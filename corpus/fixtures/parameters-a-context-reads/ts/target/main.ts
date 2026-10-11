// mutate infers its variables' type from the function it is handed.
function mutate<T>(fn: (variables: T) => number): (variables: T) => number {
  return fn;
}

// withSignal hands its callback an object whose properties the callback may read.
function withSignal(fn: (context: { signal: string; retry: number }) => number): number {
  return fn({ signal: "s", retry: 1 });
}

// register calls the handler it is handed.
function register(handler: (name: string) => number): number {
  return handler("n");
}

// The annotation types the variables the call infers.
const run = mutate(
  (
    _variables: string,
  ) => 1,
);

// The pattern reads the property it names from the argument.
const signalled = withSignal(
  (
    { signal },
  ) => 1,
);

// chat is a declared function, and no call infers anything from its annotation.
function chat(
  question: string,
): number {
  return 1;
}

// handle is a function expression no context types where it is written.
const handle = (
  label: string,
): number => 1;

function pair(): [number, number] {
  return [1, 2];
}

// first holds an array pattern whose first element nothing reads.
function first(): number {
  const [
    unread,
    used,
  ] = pair();
  return used;
}

export const results = [run("v"), signalled, chat("q"), register(handle), first()];
