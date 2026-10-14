declare global {
  interface Window {
    chrome?: unknown;
  }
}

export const kind = typeof window;
export const strict = import.meta.strict;
