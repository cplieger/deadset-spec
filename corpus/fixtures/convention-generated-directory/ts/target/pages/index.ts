import { routeName } from "../.nuxt/routes.js";
import { used } from "../util.js";

if (routeName.length + used === 0) {
  throw new Error("nothing was named");
}
