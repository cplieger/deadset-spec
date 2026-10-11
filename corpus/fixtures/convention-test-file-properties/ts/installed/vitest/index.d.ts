export interface ProvidedContext {}

export declare function inject<K extends keyof ProvidedContext>(key: K): ProvidedContext[K];
