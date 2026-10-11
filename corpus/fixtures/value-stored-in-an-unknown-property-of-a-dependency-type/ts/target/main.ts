import { apiAction } from "@example/actions";

interface CreateRequest {
  name: string;
  source?: string;
}

interface LocalRequest {
  body?: unknown;
}

interface Unsent {
  label: string;
}

const create = apiAction<CreateRequest>({
  request: (body) => ({ method: "POST", path: "/api/tools", body }),
});

function submit(request: CreateRequest): Promise<void> {
  return create.dispatch(request);
}

const local: LocalRequest = {};
const unsent: Unsent = { label: "u" };
local.body = unsent;

export const sent: Promise<void> = submit({ name: "tool", source: "catalog" });
export const held: boolean = local.body !== undefined;
