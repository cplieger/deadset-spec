import { Clock, Report } from "./services.js";

// The entry file lists both classes for a container, so each is used as a value.
const services = [Clock, Report];
console.log(services.length);
