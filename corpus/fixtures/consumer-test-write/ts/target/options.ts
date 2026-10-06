// Options holds the settings a caller passes on.
export interface Options {
  label?: string;
}

// defaults returns empty options.
export function defaults(): Options {
  return {};
}
