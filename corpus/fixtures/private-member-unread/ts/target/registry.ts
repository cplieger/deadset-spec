// The class whose three private members carry the fixture's expectations.
export class Registry {
  // Written in the constructor and read only through a string index.
  private label: string;

  // Written in the constructor and read nowhere.
  #writes = 0;

  // Named nowhere at all.
  #origin?: string;

  constructor(label: string) {
    this.label = label;
    this.#writes = 1;
  }
}
