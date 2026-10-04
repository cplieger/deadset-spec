// project is the only part of Handle that names P, and nothing writes or reads it.
export interface Handle<P> {
  readonly id: number;
  readonly project?: P;
}

// label names no type parameter, and nothing writes or reads it.
export interface Note {
  readonly id: number;
  readonly label?: string;
}

// claims is the only part of Ledger that names F, and an object literal writes it.
export interface Ledger<F> {
  readonly total: number;
  readonly claims: readonly F[];
}

export function open<P>(id: number): Handle<P> {
  return { id };
}

export function note(id: number): Note {
  return { id };
}

export function ledgerOf<F>(claims: readonly F[]): Ledger<F> {
  return { total: claims.length, claims };
}
