// A class whose constructor declares five parameter properties and one plain parameter.
export class Box {
  constructor(
    private readonly label: string,
    private readonly spare: string,
    public readonly width: number,
    public readonly height: number,
    public readonly depth: number,
    count: number,
  ) {
    console.log("box");
  }

  describe(): string {
    return this.label;
  }
}

// A class the entry file uses as a value.
export class Shelf {
  constructor(size: number) {
    console.log("shelf");
  }
}
