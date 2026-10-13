// Types the files a ?raw specifier names, which a bundler hands over as text.

declare module "*?raw" {
  const contents: string;
  export default contents;
}

// No specifier of the program matches this pattern.
declare module "*?inline" {
  const inlined: string;
  export default inlined;
}
