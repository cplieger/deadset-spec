type HandleArgs<Unread> = { n: number };

type Boxed<Value> = { value: Value };

interface Tagged<Tag> {
  readonly n: number;
}

const args: HandleArgs<string> = { n: 1 };
const boxed: Boxed<number> = { value: 2 };
const tagged: Tagged<string> = { n: 3 };

console.log(args.n, boxed.value, tagged.n);
