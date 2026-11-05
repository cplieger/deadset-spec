import { routeName } from "../.nuxt/routes.js";
import { header } from "../src/header.js";
import { notice } from "../src/notice.js";
import { tagged } from "../src/tagged.js";
import { formatted } from "../vendor/format.min.js";

console.log(routeName, header, notice, tagged, formatted);
