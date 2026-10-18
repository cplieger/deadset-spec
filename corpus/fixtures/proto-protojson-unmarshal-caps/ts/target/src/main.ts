import { fromJson } from "@bufbuild/protobuf";
import { PingRequestSchema, PingResponseSchema } from "./gen/v1/service_pb.js";

const request = fromJson(PingRequestSchema, {});
const response = fromJson(PingResponseSchema, {}, { ignoreUnknownFields: true });
console.log(request.number, response === undefined);
