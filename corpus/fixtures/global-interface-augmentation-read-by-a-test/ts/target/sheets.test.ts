const sheets = import.meta.glob("./css/*.css");
const value = ownGlobal.read();

export function count(): number {
  return Object.keys(sheets).length + value.length;
}
