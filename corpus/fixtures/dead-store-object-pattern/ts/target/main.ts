interface Meta {
  readonly request: number;
  readonly response: number;
  readonly extra: number;
}

// pick reads request, the rest and second, and none of the other names.
function pick(meta: Meta): number {
  const {
    request,
    response,
  } = meta;
  const {
    request: first,
    ...rest
  } = meta;
  const {
    extra = 1,
    response: second,
  } = meta;
  return request + rest.response + second;
}

export const picked = pick({ request: 1, response: 2, extra: 3 });
