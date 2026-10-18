import { create } from "@bufbuild/protobuf";
import { PingRequestSchema } from "./gen/v1/service_pb.js";

const request = create(PingRequestSchema, { text: "a" });
console.log(request.number);
