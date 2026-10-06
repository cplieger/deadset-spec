// parse reads its input as JSON and falls back to an empty object, reading
// nothing of what it caught.
function parse(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch (error) {
    return {};
  }
}

// parseOrReport returns the error it catches.
function parseOrReport(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch (failure) {
    return { failure };
  }
}

// parseQuietly writes its catch clause with no binding.
function parseQuietly(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

if (parse("{}") === parseOrReport("[]") || parseQuietly("1") === 2) {
  throw new Error("the parsers agree");
}
