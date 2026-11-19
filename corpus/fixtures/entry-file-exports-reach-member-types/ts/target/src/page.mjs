// A file only the src of an HTML script loads: the page runs it and reads none of its exports.
class Page {
  draw() {
    return 0;
  }
}

export const page = { make: Page };
