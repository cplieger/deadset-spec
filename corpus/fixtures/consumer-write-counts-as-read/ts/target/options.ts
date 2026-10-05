// Options holds the settings a caller passes on.
export interface Options {
  label?: string;
  note?: string;
}

// defaults returns the options with a note set.
export function defaults(): Options {
  return { note: "default" };
}
