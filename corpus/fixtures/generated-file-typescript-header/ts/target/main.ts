import { ready } from "./late.js";
import { usedMessage } from "./schema_pb.js";
import { usedRequest } from "./wire.js";

// The entry reads one export of each of the three files.
export const loaded = [ready, usedMessage, usedRequest()];
