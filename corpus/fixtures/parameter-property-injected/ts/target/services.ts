// A parameter decorator a container reads to choose what it injects.
function Inject(token: string) {
  return (_target: object, _key: string | undefined, index: number): void => {
    console.log(token, index);
  };
}

// A class a container constructs, because a parameter of its constructor carries a decorator.
export class Clock {
  constructor(
    @Inject("zone") private readonly zone: string,
    private readonly ticks: number,
    retries: number,
  ) {
    console.log("clock");
  }
}

// A class with no decorator anywhere, whose parameter property nothing reads.
export class Report {
  constructor(private readonly title: string) {
    console.log("report");
  }
}
