const first = { testRunner: "jest" };
module.exports = first;
exports = module.exports = { testRunner: "mocha" };
module.exports.testRunner = "vitest";
