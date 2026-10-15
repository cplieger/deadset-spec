const registry = new Map<string, unknown>();

function require(name: string): unknown {
  return registry.get(name);
}

function calledOnlyByDead(): void {}

function deadCaller(): void {
  calledOnlyByDead();
}

const name = String(Date.now());
console.log(require(name));
