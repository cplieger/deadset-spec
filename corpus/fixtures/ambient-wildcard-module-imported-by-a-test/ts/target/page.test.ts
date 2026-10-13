import page from "./page.html?raw";

// A test file under the default test-file pattern, which reads a non-code file as text.
export function testPage(): void {
  if (page.length === 0) {
    throw new Error("the page is empty");
  }
}
