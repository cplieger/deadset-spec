const require = (name) => () => name;

require("webpack")({ entry: "./src/unpacked.ts" });
