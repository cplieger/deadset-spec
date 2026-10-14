// The bundler gives every module this member at build time.

interface ImportMeta {
  glob(pattern: string): Record<string, string>;
}

// No library declares this interface.
interface OwnGlobal {
  read(): string;
}

declare const ownGlobal: OwnGlobal;
