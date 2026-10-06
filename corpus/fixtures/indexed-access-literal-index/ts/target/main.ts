import type { Operations } from "./operations.js";

// A literal index names one member.
type Listing = Operations["listUsers"];

// A union of literal indexes names each member it holds.
type Change = Operations["renameUser" | "suspendUser"];

// The index is a type parameter, so each call names the member its type argument fixes.
function result<K extends keyof Operations>(name: K, value: Operations[K]): Operations[K] {
  console.log(name);
  return value;
}

const listing: Listing = ["ada"];
const change: Change = true;
const user = result("getUser", "ada");
const removed = result<"removeUser">("removeUser", 1);
console.log(listing, change, user, removed);
