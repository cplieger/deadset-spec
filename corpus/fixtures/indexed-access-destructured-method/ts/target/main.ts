import * as store from "./store.js";

export const title = store.get("title");
export const body = store.get("body");
export const drafted = store.has("draft");
