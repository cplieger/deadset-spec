export class Scan {
  members: Record<string, number>;
  total = 0;

  constructor(members: Record<string, number>) {
    this.members = members;
  }

  add(key: string): void {
    this.members[key] = key.length;
    this.total = key.length;
  }
}
