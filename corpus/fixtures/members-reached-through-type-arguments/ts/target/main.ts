interface Result {
  code?: string;
  extra?: string;
}

interface Reply {
  code?: string;
}

interface Holder {
  tick(): void;
}

interface View {
  tick(): void;
  idle(): void;
}

async function send(result: Result, views: readonly View[]): Promise<void> {
  const answer: () => Promise<Reply | null> = () => Promise.resolve(result);
  console.log((await answer())?.code);
  for (const holder of new Set<Holder>(views)) {
    holder.tick();
  }
}

await send({ code: "a" }, [{ tick: () => undefined, idle: () => undefined }]);
