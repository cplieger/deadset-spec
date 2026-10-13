// The directory import a bundler adds to import.meta, which the program declares itself.
interface ImportMeta {
  glob(pattern: string): Record<string, string>;
}
