// A root-absolute glob reads from the project root, the directory of vite.config.ts.
export const plugins = import.meta.glob("/src/plugins/*.ts", { eager: true });
// A relative glob reads from its base, relative to this file or to the project root.
export const pages = import.meta.glob("./*.ts", { base: "./pages", eager: true });
export const routes = import.meta.glob("./*.ts", { base: "/src/routes", eager: true });
// A negated glob removes what it matches.
export const mods = import.meta.glob(["/src/mods/*.ts", "!**/skip.ts"], { eager: true });
