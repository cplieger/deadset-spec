type Args = {
  port?: number;
  seed?: string;
};

type Other = {
  unread?: number;
};

function command(args: { [key: string]: unknown }): string[] {
  return Object.keys(args).map((key) => `--${key}=${String(args[key])}`);
}

function start(args: { port?: number; [key: string]: unknown }): string[] {
  return command(args);
}

function lookup(other: { port?: number; [key: string]: unknown }): unknown {
  return other["port"];
}

const args: Args = {};
args.port = 1;
args.seed = "s";
const other: Other = {};
other.unread = 2;
export const out = [start(args), lookup(other)];
