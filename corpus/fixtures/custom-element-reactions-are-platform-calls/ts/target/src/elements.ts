const WATCHED = ["size"];
const TRACKED = ["mode"];
function probeAttr(name: string): string { return name; }
function stateOf(state: unknown): string { return String(state); }

export class SizeBox extends HTMLElement {
  static get observedAttributes(): readonly string[] { return WATCHED; }
  attributeChangedCallback(name: string): void { console.log(probeAttr(name)); }
  unused(): void {}
}

export class Field extends HTMLElement {
  static readonly observedAttributes = TRACKED;
  static formAssociated = true;
  attributeChangedCallback(): void {}
  formStateRestoreCallback(state: unknown): void { console.log(stateOf(state)); }
}

export class Base extends HTMLElement {
  connectedCallback(): void {}
}

export class Leaf extends Base {
  adoptedCallback(): void {}
}

export class Unregistered extends HTMLElement {
  connectedCallback(): void {}
}

export class Sub extends SizeBox {
  connectedCallback(): void {}
}
