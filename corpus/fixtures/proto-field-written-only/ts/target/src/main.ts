import { create } from "@bufbuild/protobuf";
import { PingRequestSchema } from "./gen/v1/service_pb.js";

export const request = create(PingRequestSchema, { number: 1 });
request.text = "a";
