// Declarations of the protobuf runtime the generated code and its callers name.
export type Message<TypeName extends string = string> = { readonly $typeName: TypeName };
export interface DescMessage { readonly typeName: string }
export type MessageShape<Desc> = Desc extends { readonly $shape?: infer Shape } ? Shape : never;
type Init<T> = T extends Message ? { [K in Exclude<keyof T, "$typeName">]?: Init<T[K]> } : T;
export type MessageInitShape<Desc> = Init<MessageShape<Desc>>;
export declare function create<Desc extends DescMessage>(schema: Desc, init?: MessageInitShape<Desc>): MessageShape<Desc>;
export declare function toBinary<Desc extends DescMessage>(schema: Desc, message: MessageShape<Desc>): Uint8Array;
export declare function fromBinary<Desc extends DescMessage>(schema: Desc, bytes: Uint8Array): MessageShape<Desc>;
export declare function fromJson<Desc extends DescMessage>(schema: Desc, json: unknown, options?: { ignoreUnknownFields?: boolean }): MessageShape<Desc>;
export declare function toJson<Desc extends DescMessage>(schema: Desc, message: MessageShape<Desc>): unknown;
export declare function equals<Desc extends DescMessage>(schema: Desc, a: MessageShape<Desc>, b: MessageShape<Desc>): boolean;
export declare const proto3: any;
export type BinaryReadOptions = any;
export type FieldList = any;
export type JsonReadOptions = any;
export type JsonValue = any;
export type PartialMessage<T> = Partial<T>;
export type PlainMessage<T> = T;
