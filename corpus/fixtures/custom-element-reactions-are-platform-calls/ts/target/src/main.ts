import { Base, Field, Leaf, SizeBox, Sub, Unregistered } from "./elements.js";

const Reached = Field;
customElements.define("size-box", SizeBox);
customElements.define("x-field", Reached);
customElements.define("x-leaf", Leaf);

export const all = [Base, Sub, Unregistered];
