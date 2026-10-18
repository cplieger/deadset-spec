import { createClient, type Transport } from "@connectrpc/connect";
import { PingService } from "./gen/v1/service_pb.js";

export async function ping(transport: Transport): Promise<void> {
  await createClient(PingService, transport).ping({ number: 1, text: "a" });
}
