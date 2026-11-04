// Vite's glob import, as vite/client declares it.
interface ImportMeta {
  glob(pattern: string | readonly string[], options?: { readonly eager?: boolean; readonly base?: string }): Record<string, unknown>;
}
