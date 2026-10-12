import type { Provider } from "@example/auth/providers";
import { github } from "@example/auth/providers/github";

function orphan(): number {
  return 2;
}

export const label: string = github;
export type Used = Provider;
