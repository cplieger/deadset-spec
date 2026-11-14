// Declarations of the protobuf runtime's code generation helpers.
import type { DescMessage } from "./index.js";
export interface GenFile { readonly kind: "file" }
export interface GenMessage<Shape> extends DescMessage { readonly $shape?: Shape }
export interface GenService<Methods> { readonly typeName: string; readonly method: Methods }
export declare function fileDesc(base64: string, deps?: GenFile[]): GenFile;
export declare function messageDesc(file: GenFile, index: number, ...path: number[]): GenMessage<any>;
export declare function serviceDesc(file: GenFile, index: number): GenService<any>;
