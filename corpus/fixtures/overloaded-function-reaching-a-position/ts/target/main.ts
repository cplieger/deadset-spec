// Key is what the comparator's first overload reads.
interface Key {
  order: number;
}
// Tab is the type of the values the sort passes the comparator.
interface Tab {
  order: number;
  title: string;
}
function byOrder(a: Key, b: Key): number;
function byOrder(a: string, b: string): number;
function byOrder(a: any, b: any): number {
  return a.order - b.order;
}
const tabs: Tab[] = [{ order: 2, title: "b" }];
console.log([...tabs].sort(byOrder).length);
