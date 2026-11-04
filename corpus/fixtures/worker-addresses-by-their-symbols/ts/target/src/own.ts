class Worker {
  constructor(path: string) {
    console.log(path);
  }
}

export const own = new Worker("./pool/owned.ts");
