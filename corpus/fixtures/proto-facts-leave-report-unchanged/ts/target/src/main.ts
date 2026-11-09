import { create } from "@bufbuild/protobuf";
import { PingRequestSchema } from "./gen/v1/service_pb.js";
import { Legacy } from "./legacy/v1/legacy_pb.js";

console.log(create(PingRequestSchema, { number: 1 }).number, new Legacy().number);
