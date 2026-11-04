function mergeConfig(base: object, more: object): object {
  return { ...base, ...more };
}

const base = { resolve: { conditions: ["browser"] } };

export default mergeConfig(base, { test: { setupFiles: ["./setup/merged.ts"] } });
