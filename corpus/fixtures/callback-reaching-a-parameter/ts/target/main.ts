interface Init {
  used: string;
  extra: string;
}
function invoke(callback: (init: Init) => string): string {
  return callback({ used: "u", extra: "e" });
}
interface Named {
  kept: string;
  spare: string;
}
const readKept = (named: Named): string => named.kept;
function relay(callback: (named: Named) => string): string {
  return callback({ kept: "k", spare: "s" });
}
interface Aliased {
  first: string;
  second: string;
}
function through(callback: (aliased: Aliased) => string): string {
  return callback({ first: "f", second: "s" });
}
const alias = through;
export const done = [invoke((init) => init.used), relay(readKept), alias((a) => a.first)];
