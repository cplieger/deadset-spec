// Declarations of the protobuf runtime's binary reader and writer.
export declare class BinaryWriter {
  uint32(value: number): this;
  int32(value: number): this;
  int64(value: bigint | number | string): this;
  bool(value: boolean): this;
  string(value: string): this;
  bytes(value: Uint8Array): this;
  double(value: number): this;
  fork(): this;
  join(): this;
  finish(): Uint8Array;
}
export declare class BinaryReader {
  constructor(bytes: Uint8Array);
  readonly pos: number;
  readonly len: number;
  uint32(): number;
  int32(): number;
  int64(): bigint;
  bool(): boolean;
  string(): string;
  bytes(): Uint8Array;
  double(): number;
  skip(wireType: number, fieldNo?: number): Uint8Array;
}
