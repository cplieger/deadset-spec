interface Payloads {
  ordinary: string;
  changed: undefined;
  switched: number;
  imported: string;
  idle: boolean;
}
export interface Frame {
  type: keyof Payloads;
}
export const selected: keyof Payloads = Math.random() > 0.5 ? "imported" : "idle";
