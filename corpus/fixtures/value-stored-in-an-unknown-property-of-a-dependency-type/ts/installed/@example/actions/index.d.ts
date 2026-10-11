export type ApiRequest =
  | { readonly method: "GET"; readonly path: string }
  | { readonly method: "POST"; readonly path: string; readonly body?: unknown };

export interface Action<A> {
  dispatch(args: A): Promise<void>;
}

export declare function apiAction<A>(options: { request: (args: A) => ApiRequest }): Action<A>;
