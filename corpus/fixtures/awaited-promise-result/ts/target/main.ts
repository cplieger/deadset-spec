function work(): Promise<number> {
  return Promise.resolve(1);
}

function step(): Promise<number> {
  return work();
}

function count(): number {
  return 2;
}

function fire(): Promise<number> {
  return work();
}

async function run(): Promise<void> {
  await step();
  await count();
  fire();
}

await run();
