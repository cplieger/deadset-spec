interface Payload {
  open: string;
  close: number;
  idle: boolean;
}
type Kind = "open" | "close";
type Kinded<K extends keyof Payload> = { kind: K; payload: Payload[K] };
export type Any = { [K in Kind]: Kinded<K> }[Kind];
export const kindOf = (a: Any): string => a.kind + String(a.payload);
