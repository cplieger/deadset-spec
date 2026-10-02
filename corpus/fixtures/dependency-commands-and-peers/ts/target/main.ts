import { attach } from "host-plugin";
import { peer } from "peer-used";
import { run } from "used-runtime";

// The entry statement reads one export of each imported dependency.
if (run() + peer() + attach() === 0) {
  throw new Error("every dependency answered zero");
}
