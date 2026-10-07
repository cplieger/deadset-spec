export class Element {
  static properties = {};
  label = "";
  get renderRoot() {
    return this;
  }
  update() {
    return [this.constructor.styles, this.constructor.properties, this.renderRoot, this.label];
  }
}
