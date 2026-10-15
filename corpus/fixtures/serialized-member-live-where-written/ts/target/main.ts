interface Target {
  uri: string;
  body?: string;
  method: string;
}

class Stamp {
  at = 1;
  unset?: number;
  set?: number;
  constructor() {
    this.set = 2;
  }
}

const target: Target = { uri: "u", method: "GET" };
console.log(JSON.stringify(target), JSON.stringify(new Stamp()));
