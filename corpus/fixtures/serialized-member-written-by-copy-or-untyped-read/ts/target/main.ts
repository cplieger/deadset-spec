interface Saved {
  name: string;
}
interface Line {
  id: string;
}
interface Listed {
  tag: string;
}
interface Base {
  kind: string;
}
interface Spread {
  kind: string;
}
interface Assigned {
  label: string;
  note: string;
}
interface Checked {
  kept: string;
  ghost: string;
}

function load(text: string): Saved {
  return JSON.parse(text);
}

let line: Line;
line = JSON.parse("{}");
const listed: Listed[] = ["{}"].map((one) => JSON.parse(one));
const base: Base = { kind: "a" };
const spread: Spread = { ...base };
const assigned = {} as Assigned;
Object.assign(assigned, { label: "x" });
const checked = JSON.parse("{}") satisfies Checked;
const sent = { kept: "k" } as Checked;
console.log(JSON.stringify(load("{}")), JSON.stringify(line), JSON.stringify(listed));
console.log(JSON.stringify(spread), JSON.stringify(assigned), JSON.stringify(sent), checked);
