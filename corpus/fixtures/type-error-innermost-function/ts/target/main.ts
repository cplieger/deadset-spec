// suite calls each body it is given.
function suite(name: string, body: () => void): void {
  if (name !== "") {
    body();
  }
}

suite("outer", () => {
  suite("evaluated", () => {
    try {
      suite("", () => {});
    } catch (unread) {
      return;
    }
  });
  suite("skipped", () => {
    try {
      const n: number = "x";
      suite(String(n), () => {});
    } catch (hidden) {
      return;
    }
  });
});
