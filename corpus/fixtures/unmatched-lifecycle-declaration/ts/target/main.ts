import { Widget } from "./widgets.js";

// The entry file creates a component; the framework calls its lifecycle member.
if (!(new Widget() instanceof Widget)) {
  throw new Error("the widget is not a widget");
}
