import { Element } from "@example/elements";

// Card overrides a static property, an accessor and an instance property the dependency's class reads.
class Card extends Element {
  static override styles = "card";

  static override properties = { title: {} };

  protected override get renderRoot(): object {
    return {};
  }

  override label = "card";

  // palette overrides nothing, and nothing reads it.
  static palette = "blue";
}

// Orphan overrides styles too, but nothing builds it.
class Orphan extends Element {
  static override styles = "orphan";
}

new Card().update();
