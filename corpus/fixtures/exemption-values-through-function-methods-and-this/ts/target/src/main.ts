export class ByCall extends HTMLElement {
  connectedCallback(): void {}
}

export class ByApply extends HTMLElement {
  connectedCallback(): void {}
}

export class ByBind extends HTMLElement {
  connectedCallback(): void {}
}

export class ByWrapper extends HTMLElement {
  connectedCallback(): void {}
}

export class ByLeft extends HTMLElement {
  connectedCallback(): void {}
}

export class ByRight extends HTMLElement {
  connectedCallback(): void {}
}

export class Record {
  key = "";
  latest(): this {
    return this;
  }
  save(): string {
    return JSON.stringify(this.latest());
  }
}

function install(name: string, element: CustomElementConstructor): void {
  customElements.define(name, element);
}

customElements.define.call(customElements, "x-call", ByCall);
customElements.define.apply(customElements, ["x-apply", ByApply]);
customElements.define.bind(customElements, "x-bind")(ByBind);
install.call(undefined, "x-wrapper", ByWrapper);
customElements.define("x-and", ByLeft && ByRight);
new Record().save();
