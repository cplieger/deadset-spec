import { version } from "gen";

if (version === "") {
  throw new Error("no version");
}
