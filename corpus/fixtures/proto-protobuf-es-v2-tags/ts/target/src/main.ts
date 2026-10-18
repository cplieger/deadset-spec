import { create } from "@bufbuild/protobuf";
import { PingRequestSchema, PingService } from "./gen/v1/service_pb.js";

const request = create(PingRequestSchema, { number: 1 });
console.log(request.number, PingService.typeName);
