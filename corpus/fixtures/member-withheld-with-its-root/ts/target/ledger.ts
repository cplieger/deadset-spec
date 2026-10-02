// live is called by the entry file.
export function live(): number {
  return 1;
}

// withheld is exported and called by nothing; the directive withholds it.
// deadset:ignore DS1001 -- Kept for a caller outside the package.
export function withheld(): number {
  return balance();
}

// balance is called by withheld alone, so it falls with it.
function balance(): number {
  return 2;
}

// reported is exported and called by nothing.
export function reported(): number {
  return audit();
}

// audit is called by reported alone, so it falls with it.
function audit(): number {
  return 3;
}
