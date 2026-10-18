import { PingServiceClientImpl } from "./gen/v1/service.js";

const client = new PingServiceClientImpl({ request: async () => new Uint8Array() });
void client.Ping({ number: 1, text: "" });
