// Row is the type a satisfies expression checks the table against.
interface Row {
  readonly key: string;
  readonly label: string;
  readonly spare?: string;
}

const ROWS = [{ key: "a", label: "A" }] satisfies readonly Row[];

// The entry reads the label through the table's own literal type.
export const firstLabel = ROWS[0]?.label;
