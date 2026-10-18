// Declarations of the Connect runtime a client names.
import type { MessageInitShape, MessageShape } from "@bufbuild/protobuf";
export interface Transport { readonly kind: "transport" }
type Method = { readonly input: any; readonly output: any };
export type Client<S extends { readonly method: Record<string, Method> }> = {
  [K in keyof S["method"]]: (request: MessageInitShape<S["method"][K]["input"]>) => Promise<MessageShape<S["method"][K]["output"]>>;
};
export declare function createClient<S extends { readonly method: Record<string, Method> }>(service: S, transport: Transport): Client<S>;
