export function pick(args: string[]): string {
  let label = "none";
  if (args.length > 1) {
    label = "some";
  } else {
    label = "empty";
  }
  let kept = "kept";
  if (args.length > 2) {
    kept = "many";
  }
  return label + kept;
}

pick([]);
