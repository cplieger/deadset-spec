// helper is called by the application.
export function helper(): number {
  return 4;
}

// idle is an export of a private member, and nothing calls it.
export function idle(): number {
  return 5;
}
