import { Element } from "@example/elements";

// Greeting extends a class a dependency declares, whose own code calls render.
class Greeting extends Element {
  protected override render(): string {
    return "hello";
  }

  // shout overrides nothing, and nothing calls it.
  shout(): string {
    return "HELLO";
  }
}

// Leaf overrides a member of the dependency's class through a class of the target.
class Base extends Element {}

class Leaf extends Base {
  override connected(): void {
    console.log("connected");
  }
}

// Orphan overrides render too, but nothing builds it.
class Orphan extends Element {
  protected override render(): string {
    return "orphan";
  }
}

new Greeting().update();
new Leaf().update();
