export function Live(): number {
  return 0;
}

export function Legacy(): number {
  return shared();
}

export function Shared(): number {
  return shared();
}

function shared(): number {
  return 1;
}
