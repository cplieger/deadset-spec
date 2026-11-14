import * as letted from "./letted.js";
import * as spelled from "./spelled.js";

// A let binding given the namespace object holds it whole.
let alias = letted;
console.log(alias.shownByLet());

// A property name and a statement label spelled as the binding are no use of it.
console.log(spelled.shownBySpelling(), { spelled: 1 });
spelled: for (;;) {
  break spelled;
}
