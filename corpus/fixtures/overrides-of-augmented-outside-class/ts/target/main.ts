import "./augment.js";
import { Element } from "@example/elements";

// Card extends the augmented class, whose own code calls render.
class Card extends Element {
  protected override render(): string {
    return this.describe();
  }

  override describe(): string {
    return "a card";
  }

  // shout overrides nothing, and nothing calls it.
  shout(): string {
    return "CARD";
  }
}

new Card().update();
