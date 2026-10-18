import { configured } from "./config.js";

export function setUp(): string {
  return configured;
}

function setUpLocal(): string {
  return "local";
}
