import { create, toBinary } from "@bufbuild/protobuf";
import { EnvelopeSchema, OtherSchema } from "./gen/v1/envelope_pb.js";

const bytes = toBinary(EnvelopeSchema, create(EnvelopeSchema, { item: { number: 1 } }));
console.log(bytes.length);
void create(OtherSchema, { number: 2 });
