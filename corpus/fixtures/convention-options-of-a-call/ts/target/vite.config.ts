import { other } from "@example/other-plugin";
import { vitePlugin as remix } from "@remix-run/dev";

const elsewhere = "elsewhere";

export default {
  plugins: [remix({ appDirectory: "src/app" }), other({ appDirectory: "elsewhere" }), other({ appDirectory: elsewhere })],
};
