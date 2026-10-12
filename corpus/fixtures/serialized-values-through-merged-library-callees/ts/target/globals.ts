// A script file: each interface merges into the library's global of that name.
interface Console {
  log(value: { id: number }, tag: "typed"): void;
}
interface CallableFunction {
  call(this: Function, thisArg: unknown, tag: "typed"): void;
}
