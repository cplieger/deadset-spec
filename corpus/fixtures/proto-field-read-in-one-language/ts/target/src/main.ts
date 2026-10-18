import { create } from "@bufbuild/protobuf";
import { PingRequestSchema } from "./gen/v1/service_pb.js";

const request = create(PingRequestSchema, { number: 1, text: "a" });
console.log(request.text);
