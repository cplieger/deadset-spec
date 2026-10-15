import { fetchJson } from "./outside.js";

interface Parsed {
  used: string;
  unread: string;
}

interface Fetched {
  used: string;
  unread: string;
}

const parsed = JSON.parse("{}") as Parsed;
const fetched = await fetchJson<Fetched>("https://example.com/");
console.log(parsed.used, fetched.used);
