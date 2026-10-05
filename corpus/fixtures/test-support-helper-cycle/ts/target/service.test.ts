import { newService } from "./support/service.js";

if (newService() !== 11) {
  throw new Error("newService() is not 11");
}
