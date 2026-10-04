const { vitePlugin: remix } = require("@remix-run/dev");
const dev = require("@remix-run/dev");

module.exports = {
  plugins: [remix({ appDirectory: "src/app" }), dev.vitePlugin({ appDirectory: "src/second" })],
};
