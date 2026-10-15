interface HttpTarget {
  uri: string;
  body: string;
}

function fieldMasks(value: Record<string, unknown>): string[] {
  return Object.keys(value);
}

const target: HttpTarget = { uri: "https://example.invalid" } as HttpTarget;
console.log(fieldMasks(target as unknown as Record<string, unknown>).join(","));
