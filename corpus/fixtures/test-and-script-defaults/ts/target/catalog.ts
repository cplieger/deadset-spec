// inSpec is referenced from a .spec file alone.
export function inSpec(): number {
  return 1;
}

// inTests is referenced from a file under __tests__ alone.
export function inTests(): number {
  return 2;
}

// inMocks is referenced from a file under __mocks__ alone.
export function inMocks(): number {
  return 3;
}

// production is referenced from the entry file.
export function production(): number {
  return 4;
}
