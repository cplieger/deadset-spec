import { commands } from "@example/kit";
import "./augment.js";

commands.emulate();
export const files = import.meta.glob("./x/*");
