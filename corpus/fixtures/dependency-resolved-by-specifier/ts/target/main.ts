// require stands for the resolver a createRequire call returns.
declare const require: { resolve(specifier: string): string };

// The entry asks the runtime where three installed packages live.
const located = [
  import.meta.resolve("meta-resolved"),
  require.resolve("require-resolved"),
  import.meta.resolve(`subpath-resolved/package.json`),
];
console.log("never-resolved", located.join(" "));
