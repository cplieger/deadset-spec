import { emit, save } from "./outside.js";

class Frozen {
  private secret = 1;
  id = 2;
}
class Copied {
  private secret = 1;
  id = 2;
}
class Boxed {
  private secret = 1;
  id = 2;
}
class Deep {
  private secret = 1;
  id = 2;
}
class Stored {
  private secret = 1;
  id = 2;
}
class Base {
  base = 0;
  dump(): string {
    return JSON.stringify(this);
  }
}
class Derived extends Base {
  private secret = 1;
  id = 2;
}
class Precise {
  private secret = 1;
  id = 2;
}

interface Box<U> {
  v: U;
}
function deep<T>(x: Box<Box<Box<Box<T>>>>): string {
  return JSON.stringify(x);
}
class Store<T> {
  save(x: T): string {
    return JSON.stringify(x);
  }
}

console.log(JSON.stringify(Object.freeze(new Frozen())));
console.log(JSON.stringify(emit(new Copied())));
save({ box: { value: new Boxed() } });
console.log(deep({ v: { v: { v: { v: new Deep() } } } }));
console.log(new Store<Stored>().save(new Stored()));
console.log(new Derived().dump());
console.log(JSON.stringify({ id: new Precise().id }));
