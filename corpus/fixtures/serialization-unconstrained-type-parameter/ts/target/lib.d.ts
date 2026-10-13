// send hands its data to code outside the analysis.
export declare function send<D>(url: string, data?: D): Promise<void>;
export declare function keyed<D extends { readonly id: string }>(data: D): void;
