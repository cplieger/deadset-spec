interface Reading {
  level: number;
}

interface Settings {
  level: number;
  label: string;
}

export class Panel {
  #level = 0;

  get settings(): Reading {
    return { level: this.#level };
  }

  set settings(next: Settings) {
    this.#level = next.level;
  }
}
