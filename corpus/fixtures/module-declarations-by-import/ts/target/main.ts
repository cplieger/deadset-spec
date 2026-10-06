import logo from "./logo.svg";
import { routes } from "virtual:routes";
import type { Registry } from "./registry.js";

// The augmentation adds members to the registry the import above names.
declare module "./registry.js" {
  interface Registry {
    extra: number;
    unused?: string;
  }
}

const value: Registry = { base: 1, extra: 2 };
console.log(logo, routes.length, value.base + value.extra);
