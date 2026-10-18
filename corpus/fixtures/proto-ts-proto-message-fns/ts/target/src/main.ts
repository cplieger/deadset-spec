import { PingRequest, PingResponse } from "./gen/v1/service.js";

const bytes = PingRequest.encode(PingRequest.fromPartial({ number: 1 })).finish();
const response = PingResponse.fromPartial({ number: 2 });
console.log(bytes.length, response === undefined);
