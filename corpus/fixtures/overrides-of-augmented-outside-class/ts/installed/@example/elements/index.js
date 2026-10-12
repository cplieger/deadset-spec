export class Element {
  render() {
    return "";
  }
  connected() {}
  update() {
    this.connected();
    return this.render();
  }
}
