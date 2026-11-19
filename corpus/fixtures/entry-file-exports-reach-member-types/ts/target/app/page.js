// A route file the next row's entry globs name: the framework reads its exports.
class Route {
  static segment = "home";

  render() {
    return "home";
  }

  #draft() {
    return "draft";
  }
}

export const route = { handler: Route };
