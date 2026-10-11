import { createStore } from "@example/store";

interface Settings {
  [key: string]: unknown;
  page: string;
  theme: string;
}

const settings = createStore<Settings>();

export const page = settings.get("page");
export const other = settings.get("elsewhere");
