// Options is the type of the published entry's parameter.
export interface Options {
  name: string;
  plugin?: Plugin;
}

// Plugin is the type of a property of Options, which the published API reaches through it.
export interface Plugin {
  webpack?: string;
}

// Loaded is the element type of the type argument of a published result.
export interface Loaded {
  id: number;
  extra?: number;
}

// Entry is the element type of a published class's property.
export interface Entry {
  label?: string;
}
