function deadCaller(): number {
  return calledOnlyByDead();
}

function calledOnlyByDead(): number {
  return 1;
}

// Vite resolves a glob that starts with neither / nor . through an alias.
export const plugins = import.meta.glob("@/plugins/*.ts", { eager: true });
