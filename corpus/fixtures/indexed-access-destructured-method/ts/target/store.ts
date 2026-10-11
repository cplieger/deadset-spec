import { createStore } from "@example/store";

interface Fields {
  title: string;
  body: string;
  draft: boolean;
  extra: number;
}

export const { get, has } = createStore<Fields>();
