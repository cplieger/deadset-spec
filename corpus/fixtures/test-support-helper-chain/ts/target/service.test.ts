import { newService } from "./support/service.js";

if (newService() !== 4) {
  throw new Error("newService() is not 4");
}
