// Mode is named by the annotation of one write-only member alone.
type Mode = "fast" | "slow";

// Settings is built, and its name is read.
interface Settings {
  readonly mode: Mode;
  readonly name: string;
}

function load(): Settings {
  return { name: "app", mode: "fast" };
}

// The entry reads the name.
export const title = load().name;
