interface Rollout {
  name: string;
  build: number;
  notes: string;
}

type Draft<T> = { [K in keyof T]?: T[K] };

const draft: Draft<Pick<Rollout, "name">> = { name: "r" };
const extra = { build: 7 };
const rollout = Object.assign({} as Rollout, draft, extra);
console.log(JSON.stringify(rollout));
