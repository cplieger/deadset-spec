// format is imported by a component and called only in its markup.
export function format(text: string): string {
  return text.trim();
}

// unusedFormat is imported by nothing.
export function unusedFormat(text: string): string {
  return text;
}
