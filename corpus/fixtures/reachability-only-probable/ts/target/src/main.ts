function deadCaller(): number {
  return calledOnlyByDead();
}

function calledOnlyByDead(): number {
  return 1;
}

console.log(1);
