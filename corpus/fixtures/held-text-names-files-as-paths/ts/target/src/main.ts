import { current } from "./versions.js";
import { heldValue } from "./held.js";
import { send } from "./notify.js";
import { show } from "./toast/view.js";
import { act } from "./actions/index.js";

send(current() + heldValue() + show() + act());
