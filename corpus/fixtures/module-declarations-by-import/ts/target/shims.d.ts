// Each declaration below describes a module no source file of the program provides.

declare module "*.svg" {
  const url: string;
  export default url;
}

declare module "*.wasm" {
  const bytes: Uint8Array;
  export default bytes;
}

declare module "virtual:routes" {
  export const routes: string[];
}

declare module "legacy-widget" {
  export function mount(): void;
}
