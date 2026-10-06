// The test runner's configuration names a setup file to load and files to cover.
export default {
  test: {
    setupFiles: ["./src/setup.ts"],
    coverage: {
      include: ["src/covered.ts"],
      exclude: ["src/excluded.ts"],
    },
  },
};
