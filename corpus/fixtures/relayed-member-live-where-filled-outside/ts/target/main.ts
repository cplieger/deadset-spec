import { fetchJson, getJson } from "./outside.js";

interface Row {
  title: string;
}

interface Made {
  label: string;
}

interface Kept {
  used: string;
  unread: string;
}

const row = (await getJson("https://example.com/row")) as Row;
const made = await fetchJson<Made>("https://example.com/made");
const kept = (await getJson("https://example.com/kept")) as Kept;
console.log(JSON.stringify(row), JSON.stringify(made), kept.used);
