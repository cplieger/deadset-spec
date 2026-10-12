// A serializer reads toJSON with [[Get]] and calls what it answers, so a getter
// answers it as a method does; another getter is not serialized.
class Odd {
  id = 1;
  get toJSON(): () => number {
    return () => this.id;
  }
  get label(): string {
    return "odd";
  }
}
console.log(JSON.stringify(new Odd()));

interface Shape {
  id: number;
  get toJSON(): () => number;
}
const shape: Shape = {
  id: 1,
  get toJSON() {
    return () => 2;
  },
};
console.log(JSON.stringify(shape));
