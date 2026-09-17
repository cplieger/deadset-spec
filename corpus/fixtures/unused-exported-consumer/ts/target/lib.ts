// The exported function no loaded package references.
export function deadExport(): string {
  return "dead";
}

// The exported function the consumer package references.
export function usedByConsumer(): string {
  return "used";
}
