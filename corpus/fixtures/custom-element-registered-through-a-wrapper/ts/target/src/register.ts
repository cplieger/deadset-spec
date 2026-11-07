export function register(name: string, element: CustomElementConstructor): void {
  customElements.define(name, element);
}

export const registerArrow = (name: string, element: CustomElementConstructor): void =>
  customElements.define(name, element);

export class Registry {
  add(name: string, element: CustomElementConstructor): void {
    customElements.define(name, element);
  }
}

export function registerRest(name: string, ...element: [CustomElementConstructor]): void {
  customElements.define(name, ...element);
}

export function outer(element: CustomElementConstructor): void {
  register("x-two", element);
}

export function describe(element: CustomElementConstructor): string {
  return element.name;
}

export const table = {
  add: (name: string, element: CustomElementConstructor): void =>
    customElements.define(name, element),
};

export const registerExpression = function (element: CustomElementConstructor): void {
  customElements.define("x-expression", element);
};

export class Hosted {
  add(this: Hosted, element: CustomElementConstructor): void {
    customElements.define("x-this", element);
  }
}

export function registerLoose(name: string, element: unknown): void {
  customElements.define(name, element as CustomElementConstructor);
}
