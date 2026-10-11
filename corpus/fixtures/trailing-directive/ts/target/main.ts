function setup(): void {}

function run(): void {
  setup(); // deadset:ignore DS1002 -- Kept for the generator.
  const local = 1;
  console.log(local);
}

const unusedAfter = 2;

const pinned = "v1"; // deadset:ignore DS1002 -- The generator reads this value from source.

const first = 1; // deadset:ignore DS1002 -- The generator reads this value from source.
const second = 2;

// pin: datasource=git-refs name=example-list
const listCommit = "v2"; // deadset:ignore DS1002 -- The generator reads this value from source.

run();
