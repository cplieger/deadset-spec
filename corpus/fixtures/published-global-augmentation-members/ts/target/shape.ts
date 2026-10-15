declare global {
  interface Window {
    fromImport?: unknown;
  }
}

export interface Shape {
  side: number;
}
