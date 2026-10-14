function deadCaller(): number {
  return calledOnlyByDead();
}

function calledOnlyByDead(): number {
  return 1;
}

export async function start(name: string): Promise<unknown> {
  return import(name);
}

void start("a");
