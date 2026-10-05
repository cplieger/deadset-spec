// Widget is a class the published entry re-exports.
export class Widget {
  // render is a public member of the published class that nothing calls.
  render(): string {
    return "widget";
  }
}
