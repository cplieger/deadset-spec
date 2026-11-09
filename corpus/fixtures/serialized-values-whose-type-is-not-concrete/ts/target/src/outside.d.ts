export declare function emit<T extends object>(value: Readonly<T>): T;
export declare function save<T>(value: { [K in "box"]: { value: T } }): void;
