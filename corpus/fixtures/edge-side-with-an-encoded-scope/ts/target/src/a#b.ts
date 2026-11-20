// live is called below, so the side naming it is live.
function live(): number {
  return 1;
}

// dead is called by nothing, so the side naming it is dead.
function dead(): number {
  return 2;
}

// The file's export reads the result of live.
export const one = live();
