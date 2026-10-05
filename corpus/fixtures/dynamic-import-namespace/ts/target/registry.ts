// register keeps a loader whose module it reads in ways the analysis does not follow.
export function register(loader: Promise<unknown>): void {
  void loader;
}
