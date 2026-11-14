import { createFileRegistry } from "@bufbuild/protobuf";
import { file_v1_c } from "./gen/v1/c_pb.js";
import * as d from "./gen/v1/d_pb.js";
import * as g from "./gen/v1/g_pb.js";
import * as j from "./gen/v1/j_pb.js";
import * as k from "./gen/v1/k_pb.js";
import aliased = d.file_v1_d;

const { file_v1_g } = g;
const registry = createFileRegistry({ file_v1_c }, aliased, file_v1_g, j[`file_v1_j`]);
console.log(registry);

async function load(): Promise<unknown> {
  return createFileRegistry(...Object.values(await import("./gen/v1/i_pb.js")));
}
console.log(await load());

export import reexported = k.file_v1_k;
export * from "./gen/v1/e_pb.js";
export * as f from "./gen/v1/f_pb.js";
export type * from "./gen/v1/h_pb.js";
