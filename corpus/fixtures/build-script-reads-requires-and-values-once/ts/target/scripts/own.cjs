function require(name) {
  return { build() {} };
}

const esbuild = require("esbuild");
esbuild.build({ entryPoints: ["src/fake.ts"] });
