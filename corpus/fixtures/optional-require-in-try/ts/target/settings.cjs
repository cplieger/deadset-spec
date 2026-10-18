try {
  require("./local-settings.cjs");
} catch {
  // The local settings are optional.
}
const overrides = require("./overrides.cjs") || {};
console.log(overrides);
