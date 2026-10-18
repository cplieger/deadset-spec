import { createClient, type Transport } from "@connectrpc/connect";
import { PingService } from "./gen/v1/service_pb.js";

export async function ping(transport: Transport): Promise<number> {
  const client = createClient(PingService, transport);
  const response = await client.ping({ number: 1 });
  return response.number;
}
