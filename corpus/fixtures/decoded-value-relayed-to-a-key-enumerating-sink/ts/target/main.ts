interface Settings {
  theme: string;
  width: number;
}

interface Local {
  used: number;
  unused: number;
}

function dump(value: object): string {
  return Object.entries(value).map(([key, v]) => `${key}=${String(v)}`).join(",");
}

const settings = JSON.parse("{}") as Settings;
const local: Local = { used: 1, unused: 2 };
console.log(dump(settings), local.used);
