import { createFileRegistry } from "@bufbuild/protobuf";
import { file_v1_a } from "./gen/v1/a_pb.js";
import type { file_v1_b } from "./gen/v1/b_pb.js";

const registry = createFileRegistry(file_v1_a);
const other: typeof file_v1_b | undefined = undefined;
console.log(registry, other);
