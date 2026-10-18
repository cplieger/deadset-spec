import { create, equals } from "@bufbuild/protobuf";
import { PingRequestSchema } from "./gen/v1/service_pb.js";

const first = create(PingRequestSchema, { number: 1 });
const second = create(PingRequestSchema, { number: 2 });
console.log(equals(PingRequestSchema, first, second));
