// Bag is iterated by a for-of statement, Pair by a spread.
class Bag {
  [Symbol.iterator](): Iterator<number> {
    return [1].values();
  }
  [Symbol.asyncIterator](): AsyncIterator<number> {
    return (async function* () {
      yield 1;
    })();
  }
}

class Pair {
  [Symbol.iterator](): Iterator<number> {
    return [1, 2].values();
  }
}

let total = 0;
for (const n of new Bag()) {
  total += n;
}
export const all = [...new Pair(), total];
